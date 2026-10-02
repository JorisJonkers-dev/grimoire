package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

func TestInventoriesAreStoredWithTheirTransfers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	if _, err := comppg.New(tb.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope", Description: "Rope."}, Category: "gear", WeightLB: 5}},
	}, "inventory"); err != nil {
		t.Fatal(err)
	}
	char := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
		ability_method, hp_max, hp_current) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10)`, char, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}
	store := pgstore.New(tb.pool)
	inv, err := store.LoadInventory(ctx, tb.campaign)
	if err != nil || len(inv.Containers) != 2 || len(inv.Bearers) != 1 || inv.Bearers[0].Strength != 10 {
		t.Fatalf("inventory = %+v %v", inv, err)
	}
	if again, _ := store.LoadInventory(ctx, tb.campaign); len(again.Containers) != 2 {
		t.Fatalf("loading twice makes no second stash = %+v", again.Containers)
	}
	stash, pack := inv.Containers[0], inv.Containers[1]
	if stash.Kind != domain.ContainerStash {
		stash, pack = pack, stash
	}
	drop := domain.Container{ID: domain.ContainerID(uuid.New()), Kind: domain.ContainerDrop, Label: "Loot: Hoard", Items: map[string]int{"rope": 3}, Coins: map[string]int{"gp": 10}}
	me := tb.dmMember(t)
	writes := []live.Write{
		{Kind: domain.ActionLootDropped, Drop: &drop},
		{Kind: domain.ActionItemMoved, Move: &domain.Move{From: drop.ID, To: pack.ID, Item: "rope", Count: 2, Left: 1, Now: 2, FromLabel: "Loot: Hoard", ToLabel: "Aria"}},
		{Kind: domain.ActionCoinsMoved, Move: &domain.Move{From: drop.ID, To: stash.ID, Coin: "gp", Count: 10, Left: 0, Now: 10}},
		{Kind: domain.ActionItemMoved, Move: &domain.Move{From: drop.ID, To: stash.ID, Item: "rope", Count: 1, Left: 0, Now: 1}, Gone: &drop.ID},
		{Kind: domain.ActionCoinsMoved, Move: &domain.Move{From: stash.ID, To: pack.ID, Coin: "gp", Count: 4, Left: 6, Now: 4}},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, s, nil, w, me, dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	inv, err = store.LoadInventory(ctx, tb.campaign)
	if err != nil || len(inv.Containers) != 2 || inv.Items["rope"].WeightLb != 5 {
		t.Fatalf("after = %+v %v", inv, err)
	}
	for _, c := range inv.Containers {
		want := map[string]int{"rope": 1}
		coins := 6
		if c.Kind == domain.ContainerCharacter {
			want, coins = map[string]int{"rope": 2}, 4
		}
		if c.Items["rope"] != want["rope"] || c.Coins["gp"] != coins {
			t.Fatalf("%s = %+v", c.Kind, c)
		}
	}
	if items, err := store.Items(ctx, tb.campaign, nil); err != nil || len(items) != 0 {
		t.Fatalf("no slugs = %+v %v", items, err)
	}
	fresh := func() *domain.Container {
		c := drop
		c.ID = domain.ContainerID(uuid.New())
		return &c
	}
	hero := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Kind: domain.TokenParty, Stats: &domain.Stats{Source: "character:" + char.String(), AC: 16, HP: 10, HPMax: 10}}
	if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionTokenPlaced, Token: hero}, me, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(repo *pgstore.Store) error{
		"load":  func(repo *pgstore.Store) error { _, err := repo.LoadInventory(ctx, tb.campaign); return err },
		"items": func(repo *pgstore.Store) error { _, err := repo.Items(ctx, tb.campaign, []string{"rope"}); return err },
		"loot":  func(repo *pgstore.Store) error { _, err := repo.LoadLoot(ctx, tb.campaign); return err },
		"drop": func(repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionLootDropped, Drop: fresh()}, me, dm, time.Now())
			return err
		},
		"move": func(repo *pgstore.Store) error {
			mv := &domain.Move{From: stash.ID, To: pack.ID, Item: "rope", Count: 1, Left: 1, Now: 2}
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionItemMoved, Move: mv}, me, dm, time.Now())
			return err
		},
		"swap": func(repo *pgstore.Store) error {
			hero.Stats.Attacks = []domain.Attack{{Name: "Longbow", ToHit: 4, RangeFt: 150, LongRangeFt: 600, Damage: "1d8", DamageType: "piercing"}}
			sw := &live.WeaponSwap{Character: char, Set: "ranged", Armor: "chain-mail", Weapons: []string{"longbow"}}
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionObjectUsed, Token: hero, Swap: sw}, me, dm, time.Now())
			return err
		},
		"empty": func(repo *pgstore.Store) error {
			d := fresh()
			if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionLootDropped, Drop: d}, me, dm, time.Now()); err != nil {
				t.Fatal(err)
			}
			mv := &domain.Move{From: d.ID, To: stash.ID, Coin: "gp", Count: 10, Left: 0, Now: 16}
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionCoinsMoved, Move: mv, Gone: &d.ID}, me, dm, time.Now())
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

// The schema keeps Containers and Item Instances honest: every Container has exactly one owner, and an
// item singled out by a name, Charges, a slot or Attunement is one item.
func TestItemInstancesAndBagsRefuseNonsense(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	stash := uuid.New()
	container := func(id uuid.UUID, kind string, parent any) error {
		_, err := tb.pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, parent_id, label, created_at) VALUES ($1, $2, $3, $4, 'Box', now())`,
			id, tb.campaign, kind, parent)
		return err
	}
	if err := container(stash, domain.ContainerStash, nil); err != nil {
		t.Fatal(err)
	}
	bag := uuid.New()
	if err := container(bag, domain.ContainerBag, stash); err != nil {
		t.Fatalf("a bag in the stash = %v", err)
	}
	for name, err := range map[string]error{
		"a bag that sits nowhere": container(uuid.New(), domain.ContainerBag, nil),
		"a stash inside a bag":    container(uuid.New(), domain.ContainerStash, bag),
		"a bag inside itself":     container(bag, domain.ContainerBag, bag),
		"a container of no kind":  container(uuid.New(), "chest", nil),
	} {
		if err == nil {
			t.Errorf("%s was stored", name)
		}
	}
	instance := func(qty int, name, charges, slot any, identified, attuned bool) error {
		_, err := tb.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, charges, identified, attuned, equipped_slot, created_at)
			VALUES ($1, $2, 'rope', $3, $4, $5, $6, $7, $8, now())`, uuid.New(), bag, name, qty, charges, identified, attuned, slot)
		return err
	}
	if err := instance(1, "Climbing Line", 3, "neck", true, true); err != nil {
		t.Fatalf("a named, charged, worn rope = %v", err)
	}
	if err := instance(50, nil, nil, nil, false, false); err != nil {
		t.Fatalf("a plain stack = %v", err)
	}
	for name, err := range map[string]error{
		"a named stack":               instance(2, "Twins", nil, nil, true, false),
		"a charged stack":             instance(2, nil, 1, nil, true, false),
		"a worn stack":                instance(2, nil, nil, "feet", true, false),
		"an attuned stack":            instance(2, nil, nil, nil, true, true),
		"attuned yet unidentified":    instance(1, nil, nil, nil, false, true),
		"a second thing at the neck":  instance(1, nil, nil, "neck", true, false),
		"a slot nobody has":           instance(1, nil, nil, "tail", true, false),
		"more charges than the rules": instance(1, nil, 101, nil, true, false),
	} {
		if err == nil {
			t.Errorf("%s was stored", name)
		}
	}
}
