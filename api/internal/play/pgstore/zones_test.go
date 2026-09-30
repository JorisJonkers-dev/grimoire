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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func TestEncounterZonesKeepTheirCreaturesAndChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	me := tb.dmMember(t)
	lurker := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Goblin", Kind: domain.TokenEnemy, Hidden: true, Stats: &domain.Stats{Source: "monster:goblin", AC: 15, HP: 7, HPMax: 7, Stealth: 6, SpeedFt: 30}}
	hero := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Kind: domain.TokenParty}
	for _, tok := range []domain.Token{lurker, hero} {
		if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionTokenPlaced, Token: tok}, me, dm, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Perception for Aria", Notation: "1d20", RequestedBy: me.Name, Roller: me,
		Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 20}},
	}
	yes := true
	z := domain.Zone{ID: domain.ZoneID(uuid.New()), Name: "Ambush", At: hex.Coord{Q: 3, R: 0}, RadiusHexes: 2, Status: domain.ZoneArmed, Checks: []domain.ZoneCheck{}}
	sprung := z
	sprung.Status, sprung.DC, sprung.Creatures = domain.ZoneSpotting, 16, []domain.TokenID{lurker.ID}
	sprung.Checks = []domain.ZoneCheck{{Token: hero.ID, RollID: &roll.ID}}
	seen := sprung
	seen.Checks = []domain.ZoneCheck{{Token: hero.ID, RollID: &roll.ID, Noticed: &yes}}
	shown := lurker
	shown.Hidden = false
	writes := []live.Write{
		{Kind: domain.ActionZoneAdded, Zone: &z},
		{Kind: domain.ActionZoneSprung, Zone: &sprung, Rolls: []domain.Roll{roll}},
		{Kind: domain.ActionPerceptionRolled, Zone: &seen, Token: hero, Revealed: []domain.Token{shown}},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, s, nil, w, me, dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	zones, err := store.LoadZones(ctx, s.ID)
	if err != nil || len(zones) != 1 || zones[0].DC != 16 || zones[0].Status != domain.ZoneSpotting || len(zones[0].Creatures) != 1 ||
		len(zones[0].Checks) != 1 || !*zones[0].Checks[0].Noticed || *zones[0].Checks[0].RollID != roll.ID {
		t.Fatalf("zones = %+v %v", zones, err)
	}
	if _, tokens, _, _ := store.Load(ctx, s.ID); tokens[1].Hidden || tokens[1].Stats.Stealth != 6 || tokens[1].Stats.SpeedFt != 30 {
		t.Fatalf("the goblin is shown with its stealth = %+v", tokens[1])
	}
	if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionZoneRemoved, Zone: &z}, me, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	if zones, _ := store.LoadZones(ctx, s.ID); len(zones) != 0 {
		t.Fatalf("removed = %+v", zones)
	}
	for _, w := range writes[:2] {
		if _, err := store.Commit(ctx, s, nil, w, me, dm, time.Now()); err != nil && w.Kind != domain.ActionZoneSprung {
			t.Fatal(err)
		}
	}
	ops := map[string]func(repo *pgstore.Store) error{
		"load": func(repo *pgstore.Store) error { _, err := repo.LoadZones(ctx, s.ID); return err },
		"remove": func(repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionZoneRemoved, Zone: &z}, me, dm, time.Now())
			return err
		},
	}
	for _, w := range writes {
		ops[w.Kind] = func(repo *pgstore.Store) error {
			y := w
			if y.Kind == domain.ActionZoneSprung {
				fresh := roll
				fresh.ID = domain.RollID(uuid.New())
				zz := sprung
				zz.Checks = []domain.ZoneCheck{{Token: hero.ID, RollID: &fresh.ID}}
				y.Zone, y.Rolls = &zz, []domain.Roll{fresh}
			}
			_, err := repo.Commit(ctx, s, nil, y, me, dm, time.Now())
			return err
		}
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
