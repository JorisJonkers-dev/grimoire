package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// A Companion is read for the map as the Campaign keeps it, is worth what the Compendium says its
// creature is, changes hands with its token, keeps the hit points it leaves the map or the Session
// with, and XP lands on the Characters the Campaign has.
func TestCompanionsAsTheStoreKeepsThem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	stats := pgstore.Statblocks{Store: store, Characters: nil}
	goblin := snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", XP: 50, Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves: map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{}, Senses: map[string]int{}, Resistances: []string{}, Immunities: []string{},
		Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{}, Actions: []snapshot.Action{},
	}
	if _, err := comppg.New(tb.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{goblin},
	}, "companions"); err != nil {
		t.Fatal(err)
	}
	fang, bors, hero := uuid.New(), uuid.New(), uuid.New()
	for _, c := range []struct {
		id     uuid.UUID
		name   string
		who    *uuid.UUID
		shares bool
	}{{fang, "Fang", &tb.playerID, true}, {bors, "Bors", nil, false}} {
		if _, err := tb.pool.Exec(ctx, `INSERT INTO campaign.companions (id, campaign_id, name, kind, monster_slug, controller_member_id, shares_xp, created_at, updated_at)
			VALUES ($1, $2, $3, 'companion', 'goblin', $4, $5, now(), now())`, c.id, tb.campaign, c.name, c.who, c.shares); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
			ability_method, hp_max, hp_current) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10)`,
		hero, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}

	// Read for the map, and worth what the Compendium says.
	ref, err := stats.Companion(ctx, tb.campaign, fang)
	if err != nil || ref.Name != "Fang" || ref.Slug != "goblin" || ref.Controller == nil || *ref.Controller != tb.playerID || ref.HP != nil {
		t.Fatalf("Fang = %+v, %v", ref, err)
	}
	if ref, err := stats.Companion(ctx, tb.campaign, bors); err != nil || ref.Controller != nil {
		t.Fatalf("Bors = %+v, %v", ref, err)
	}
	if _, err := stats.Companion(ctx, tb.campaign, uuid.New()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("a Companion that is not there = %v", err)
	}
	if _, err := stats.Companion(ctx, uuid.New(), fang); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("a Companion of another Campaign = %v", err)
	}
	if err := stats.Creature(ctx, campaigndomain.CampaignID(tb.campaign), "goblin"); err != nil {
		t.Fatalf("a creature the Campaign has = %v", err)
	}
	if err := stats.Creature(ctx, campaigndomain.CampaignID(tb.campaign), "dragon"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("a creature the Campaign has not = %v", err)
	}
	if xp, err := store.CreatureXP(ctx, tb.campaign, "goblin"); err != nil || xp != 50 {
		t.Fatalf("a goblin is worth %d, %v", xp, err)
	}
	if xp, err := store.CreatureXP(ctx, tb.campaign, "dragon"); err != nil || xp != 0 {
		t.Fatalf("a creature nobody knows is worth %d, %v", xp, err)
	}
	for want, ids := range map[int][]uuid.UUID{1: {fang, bors}, 0: {bors}} {
		if n, err := store.SharingCompanions(ctx, tb.campaign, ids); err != nil || n != want {
			t.Fatalf("Companions with a share among %v = %d, %v", ids, n, err)
		}
	}
	if n, err := store.SharingCompanions(ctx, uuid.New(), []uuid.UUID{fang}); err != nil || n != 0 {
		t.Fatalf("a Companion of another Campaign shares = %d, %v", n, err)
	}

	sess, _ := sessions(tb, store, &closed{}).Start(ctx, dm, tb.campaign)
	actor := tb.dmMember(t)
	commit := func(s *pgstore.Store, w live.Write) error {
		_, err := s.Commit(ctx, sess, nil, w, actor, dm, time.Now())
		return err
	}
	token := func(label string, companion uuid.UUID, hp int) domain.Token {
		return domain.Token{
			ID: domain.TokenID(uuid.New()), Label: label, Kind: domain.TokenParty, Companion: &companion,
			Stats: &domain.Stats{Source: "monster:goblin", AC: 12, HP: hp, HPMax: 7, Saves: map[string]int{}, Attacks: []domain.Attack{}},
		}
	}
	onMap, staying := token("Fang", fang, 3), token("Bors", bors, 5)
	for _, tok := range []domain.Token{onMap, staying} {
		if err := commit(store, live.Write{Kind: domain.ActionTokenPlaced, Token: tok}); err != nil {
			t.Fatal(err)
		}
	}
	if _, loaded, _, err := store.Load(ctx, sess.ID); err != nil || len(loaded) != 2 || loaded[0].Companion == nil || *loaded[0].Companion != bors || *loaded[1].Companion != fang {
		t.Fatalf("tokens = %+v, %v", loaded, err)
	}

	// Every statement of handing over, awarding and leaving reports a database fault.
	handed := onMap
	handed.Controller = nil
	// A token that is nobody's Companion changes hands by itself.
	plain := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Cade", Kind: domain.TokenParty}
	if err := commit(store, live.Write{Kind: domain.ActionTokenPlaced, Token: plain}); err != nil {
		t.Fatal(err)
	}
	plain.Controller = &tb.playerID
	if err := commit(store, live.Write{Kind: domain.ActionControlAssigned, Token: plain}); err != nil {
		t.Fatal(err)
	}
	if _, loaded, _, err := store.Load(ctx, sess.ID); err != nil || loaded[1].Label != "Cade" || loaded[1].Controller == nil || *loaded[1].Controller != tb.playerID {
		t.Fatalf("Cade after changing hands = %+v, %v", loaded, err)
	}
	if err := commit(store, live.Write{Kind: domain.ActionTokenRemoved, Token: plain}); err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(s *pgstore.Store) error{
		"hand over": func(s *pgstore.Store) error {
			return commit(s, live.Write{Kind: domain.ActionControlAssigned, Token: handed})
		},
		"award": func(s *pgstore.Store) error {
			return commit(s, live.Write{Kind: domain.ActionCombatEnded, XP: []domain.XPAward{{Character: hero, Amount: 25}, {Character: uuid.New(), Amount: 25}}})
		},
		"leave": func(s *pgstore.Store) error {
			return commit(s, live.Write{Kind: domain.ActionTokenRemoved, Token: onMap})
		},
		"read": func(s *pgstore.Store) error {
			_, err := (pgstore.Statblocks{Store: s}).Companion(ctx, tb.campaign, fang)
			return err
		},
		"worth": func(s *pgstore.Store) error { _, err := s.CreatureXP(ctx, tb.campaign, "goblin"); return err },
		"sharing": func(s *pgstore.Store) error {
			_, err := s.SharingCompanions(ctx, tb.campaign, []uuid.UUID{fang})
			return err
		},
	}
	for _, name := range []string{"hand over", "award", "leave", "read", "worth", "sharing"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	// Fang went to the DM, left the map with 3 hit points, and the Character the Campaign has got the
	// XP; the award for one it has not is nobody's.
	ref, err = stats.Companion(ctx, tb.campaign, fang)
	if err != nil || ref.Controller != nil || ref.HP == nil || *ref.HP != 3 {
		t.Fatalf("Fang after leaving the map = %+v, %v", ref, err)
	}
	var xp, rows int
	if err := tb.pool.QueryRow(ctx, "SELECT (SELECT xp FROM campaign.characters WHERE id = $1), (SELECT count(*) FROM play.xp_awards WHERE character_id = $1)", hero).Scan(&xp, &rows); err != nil || xp != 25 || rows != 1 {
		t.Fatalf("the hero's XP = %d in %d awards, %v", xp, rows, err)
	}
	// Bors is still on the map when the Session ends, and keeps his 5 hit points too.
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(tb.pool, f).EndSession(ctx, tb.campaign, sess.ID, actor, dm, time.Now())
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("end: %v", err)
		}
		return err
	})
	if ref, err := stats.Companion(ctx, tb.campaign, bors); err != nil || ref.HP == nil || *ref.HP != 5 {
		t.Fatalf("Bors after the Session = %+v, %v", ref, err)
	}
}
