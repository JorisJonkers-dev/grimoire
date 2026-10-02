package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// A rest is stored with its agreements, resters and Hit Die rolls, and every failure surfaces.
func TestRestsAreStored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	char := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
		ability_method, hp_max, hp_current, level) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 20, 5, 4)`, char, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, "INSERT INTO campaign.character_resources (character_id, resource_slug, used) VALUES ($1, 'second-wind', 1)", char); err != nil {
		t.Fatal(err)
	}
	store := pgstore.New(tb.pool)
	me := tb.dmMember(t)
	info, err := store.RestInfo(ctx, tb.campaign, []uuid.UUID{char})
	if err != nil || len(info) != 1 || info[0].HitDie != 8 || info[0].HitDiceLeft != 4 || info[0].Used["second-wind"] != 1 {
		t.Fatalf("rest info without a class in the compendium = %+v %v", info, err)
	}
	token := domain.TokenID(uuid.New())
	rester := info[0]
	rester.TokenID = token
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Hit Die", Notation: "1d8", Roller: me, Status: domain.StatusPending,
		Dice: []domain.Die{{No: 0, Group: 0, Faces: 8}},
	}
	rolling := rester
	rolling.RollID = &roll.ID
	rest := &domain.Rest{Kind: live.RestShort, Status: domain.RestResting, ProposedBy: me.ID, Agreed: []uuid.UUID{me.ID}, DMAgreed: true, Resters: []domain.Rester{rolling}}
	spend := live.Write{Kind: domain.ActionHitDieSpent, Token: domain.Token{ID: token}, Rolls: []domain.Roll{roll}, Resting: rest}
	if _, err := store.Commit(ctx, s, nil, spend, me, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRest(ctx, tb.campaign, s.ID)
	if err != nil || loaded == nil || !loaded.DMAgreed || len(loaded.Agreed) != 1 || loaded.Resters[0].TokenID != token || *loaded.Resters[0].RollID != roll.ID || loaded.Resters[0].HitDiceLeft != 3 {
		t.Fatalf("loaded = %+v %v", loaded, err)
	}
	finish := live.Write{
		Kind: domain.ActionRestTaken, Rest: live.RestLong, RestOver: true,
		Results: []domain.RestResult{{CharacterID: char, HPCurrent: 99, HitDiceSpent: 0, Used: map[string]int{"second-wind": 0}, LevelUpReady: true}},
	}
	if _, err := store.Commit(ctx, s, nil, finish, me, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	if gone, err := store.LoadRest(ctx, tb.campaign, s.ID); err != nil || gone != nil {
		t.Fatalf("a finished rest is gone = %+v %v", gone, err)
	}
	var hp int
	var ready bool
	if err := tb.pool.QueryRow(ctx, "SELECT hp_current, level_up_ready FROM campaign.characters WHERE id = $1", char).Scan(&hp, &ready); err != nil || hp != 20 || !ready {
		t.Fatalf("hp %d ready %v %v", hp, ready, err)
	}
	ops := map[string]func(repo *pgstore.Store) error{
		"info": func(repo *pgstore.Store) error {
			_, err := repo.RestInfo(ctx, tb.campaign, []uuid.UUID{char})
			return err
		},
		"supplies": func(repo *pgstore.Store) error { _, err := repo.RestSupplies(ctx, tb.campaign); return err },
		"spend": func(repo *pgstore.Store) error {
			r := roll
			r.ID = domain.RollID(uuid.New())
			x := rester
			x.RollID = &r.ID
			next := *rest
			next.Resters = []domain.Rester{x}
			w := live.Write{Kind: domain.ActionHitDieSpent, Token: domain.Token{ID: token}, Rolls: []domain.Roll{r}, Resting: &next}
			_, err := repo.Commit(ctx, s, nil, w, me, dm, time.Now())
			return err
		},
		"load": func(repo *pgstore.Store) error { _, err := repo.LoadRest(ctx, tb.campaign, s.ID); return err },
		"finish": func(repo *pgstore.Store) error {
			w := finish
			w.Healed = []live.HPChange{{Token: token, Before: 1, After: 2}}
			w.Supplies = []domain.Supply{{Container: domain.ContainerID(uuid.New()), Item: "rations", Left: 0}}
			_, err := repo.Commit(ctx, s, nil, w, me, dm, time.Now())
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
