package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
)

func TestEncounterChecksAreStoredWithTheirRevisions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	goblin := snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", XP: 50, Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves: map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{}, Senses: map[string]int{}, Resistances: []string{}, Immunities: []string{},
		Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{}, Actions: []snapshot.Action{},
	}
	if _, err := comppg.New(tb.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{goblin},
	}, "checks"); err != nil {
		t.Fatal(err)
	}
	svc := &prepapp.Service{Repo: preppg.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Now: time.Now}
	pool, err := svc.SavePool(ctx, dm, tb.campaign, prep.Pool{Name: "Band", LevelMin: 1, LevelMax: 4, Difficulty: "low", Members: []prep.PoolMember{{Slug: "goblin", Weight: 1, Min: 1, Max: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	table, err := svc.SaveTable(ctx, dm, tb.campaign, prep.Table{Name: "Road", ChancePct: 50, Visibility: prep.Open, Entries: []prep.Entry{{Weight: 1, Kind: prep.EntryPool, PoolID: &pool.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	store := pgstore.New(tb.pool)
	me := tb.dmMember(t)
	sid := uuid.UUID(s.ID)
	roll := domain.Roll{
		ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Encounter check", Notation: "1d100", RequestedBy: me.Name, Roller: me,
		Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 100}},
	}
	rid := uuid.UUID(roll.ID)
	pending := prep.Check{
		ID: prep.CheckID(uuid.New()), SessionID: &sid, TableID: &table.ID, TableName: "Road", Trigger: prep.TriggerDM, Mode: prep.ModeNormal, Visibility: prep.Open,
		Seed: 42, ChancePct: 50, RollID: &rid, Status: prep.CheckPending, Monsters: []prep.EntryMonster{}, CreatedAt: time.Now(),
	}
	resolved := pending
	resolved.Status, resolved.Outcome, resolved.ChanceRoll, resolved.EntryLabel = prep.CheckResolved, prep.OutcomeFight, 12, "Band"
	resolved.Monsters = []prep.EntryMonster{{Slug: "goblin", Count: 2}}
	later := prep.Scheduled{ID: uuid.New(), TableID: table.ID, Due: prep.DueNextRest}
	writes := []live.Write{
		{Kind: domain.ActionCheckScheduled, Schedule: &later},
		{Kind: domain.ActionEncounterChecked, Check: &pending, Rolls: []domain.Roll{roll}, Unschedule: later.ID},
		{Kind: domain.ActionEncounterResolved, Check: &resolved},
		{Kind: domain.ActionRestTaken, Rest: "long"},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, s, nil, w, me, dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	checks, err := store.LoadChecks(ctx, tb.campaign, s.ID)
	if err != nil || len(checks) != 1 || checks[0].ChanceRoll != 12 || checks[0].Seed != 42 || len(checks[0].Monsters) != 1 || *checks[0].RollID != rid || *checks[0].TableID != table.ID {
		t.Fatalf("checks = %+v %v", checks, err)
	}
	p, err := store.LoadPrep(ctx, tb.campaign)
	if err != nil || len(p.Tables) != 1 || p.XP["goblin"] != 50 || len(p.Scheduled) != 0 || len(p.Levels) != 0 {
		t.Fatalf("prep = %+v %v", p, err)
	}
	if log, err := svc.Checks(ctx, dm, tb.campaign); err != nil || len(log) != 1 || log[0].Monsters[0].Count != 2 {
		t.Fatalf("log = %+v %v", log, err)
	}
	if err := svc.DeleteTable(ctx, dm, tb.campaign, table.ID); err != nil {
		t.Fatal(err)
	}
	if checks, _ := store.LoadChecks(ctx, tb.campaign, s.ID); checks[0].TableID != nil || checks[0].TableName != "Road" {
		t.Fatalf("a check outlives its table = %+v", checks[0])
	}
	if _, err := svc.RestoreTable(ctx, dm, tb.campaign, table.ID, 1); err != nil {
		t.Fatal(err)
	}
	fresh := func() live.Write {
		c := resolved
		c.ID = prep.CheckID(uuid.New())
		return live.Write{Kind: domain.ActionEncounterChecked, Check: &c}
	}
	ops := map[string]func(repo *pgstore.Store) error{
		"load prep":   func(repo *pgstore.Store) error { _, err := repo.LoadPrep(ctx, tb.campaign); return err },
		"load checks": func(repo *pgstore.Store) error { _, err := repo.LoadChecks(ctx, tb.campaign, s.ID); return err },
		"schedule": func(repo *pgstore.Store) error {
			x := prep.Scheduled{ID: uuid.New(), TableID: table.ID, Due: prep.DueNextTravel}
			_, err := repo.Commit(ctx, s, nil, live.Write{Kind: domain.ActionCheckScheduled, Schedule: &x}, me, dm, time.Now())
			return err
		},
		"check": func(repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, s, nil, fresh(), me, dm, time.Now())
			return err
		},
		"open": func(repo *pgstore.Store) error {
			w := fresh()
			r := roll
			r.ID = domain.RollID(uuid.New())
			w.Rolls, w.Unschedule = []domain.Roll{r}, uuid.New()
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
