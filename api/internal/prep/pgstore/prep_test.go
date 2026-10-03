package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	dm     = caller.UI("dm")
	player = caller.UI("player")
)

type world struct {
	pool     *pgxpool.Pool
	campaign uuid.UUID
	town     uuid.UUID
}

func creature(slug, name string, xp int) snapshot.Monster {
	return snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: name}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", ChallengeRating: 0.25, XP: xp,
		Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves:     map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{"walk": 30}, Senses: map[string]int{},
		Resistances: []string{}, Immunities: []string{}, Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{},
		Actions: []snapshot.Action{},
	}
}

func setup(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	camp := campaignapp.NewService(campaignpg.New(store.Pool()))
	d, err := camp.Create(ctx, dm, campaignapp.CreateInput{Name: "Wilds", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := camp.CreateInvite(ctx, dm, d.ID)
	if _, err := camp.AcceptInvite(ctx, player, inv.Token, "Tamsin"); err != nil {
		t.Fatal(err)
	}
	snap := snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{creature("goblin", "Goblin", 50), creature("ogre", "Ogre", 450)},
		Items: []snapshot.Item{
			{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope", Description: "Rope."}, Category: "gear", CostGP: 1, WeightLB: 5},
			{Entry: snapshot.Entry{Document: "srd-2024", Slug: "crown", Name: "Crown", Description: "Crown."}, Category: "treasure", CostGP: 5000, WeightLB: 3},
			{Entry: snapshot.Entry{Document: "srd-2024", Slug: "wand", Name: "Wand", Description: "Wand."}, Category: "wand", Magic: true, Rarity: "uncommon"},
		},
	}
	if _, err := comppg.New(store.Pool()).Import(ctx, snap, "prep"); err != nil {
		t.Fatal(err)
	}
	m, err := playpg.New(store.Pool()).InsertMap(ctx, playdomain.Map{
		CampaignID: uuid.UUID(d.ID), Name: "Realm", Kind: playdomain.MapWorld, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, Grid: "hexes", GridStrength: 20, ScaleMiles: 6, HexSize: 40, OriginX: 35, OriginY: 40,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	town := uuid.New()
	if err := queries.New(store.Pool()).InsertNode(ctx, queries.InsertNodeParams{ID: town, MapID: uuid.UUID(m.ID), Name: "Oakford"}); err != nil {
		t.Fatal(err)
	}
	return world{pool: store.Pool(), campaign: uuid.UUID(d.ID), town: town}
}

func service(w world, repo app.Repository) *app.Service {
	return &app.Service{
		Repo: repo, Members: playpg.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: func() time.Time { return time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC) },
		Seed: func() uint64 { return 9 }, Source: func(s uint64) dice.Source { return rng.New(s) },
	}
}

func goblins() domain.Pool {
	return domain.Pool{Name: " Goblin band ", LevelMin: 1, LevelMax: 4, Difficulty: "moderate", Members: []domain.PoolMember{
		{Slug: "goblin", Weight: 3, Min: 1, Max: 6}, {Slug: "ogre", Weight: 1, Min: 0, Max: 1},
	}}
}

func refused(t *testing.T, err error, want string) {
	t.Helper()
	var rule *apperr.RuleError
	if !errors.As(err, &rule) || !strings.Contains(rule.Reason, want) {
		t.Errorf("want refusal %q, got %v", want, err)
	}
}

func TestPoolsAreValidatedRevisionedAndRestorable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	s := service(w, pgstore.New(w.pool))
	p, err := s.SavePool(ctx, dm, w.campaign, goblins())
	if err != nil || p.Name != "Goblin band" || p.ID == (domain.PoolID{}) {
		t.Fatalf("create = %+v %v", p, err)
	}
	p.LevelMax, p.Difficulty = 6, "high"
	if _, err := s.SavePool(ctx, dm, w.campaign, p); err != nil {
		t.Fatal(err)
	}
	list, err := s.Pools(ctx, dm, w.campaign)
	if err != nil || len(list) != 1 || list[0].LevelMax != 6 || len(list[0].Members) != 2 || list[0].Members[0] != (domain.PoolMember{Slug: "goblin", Weight: 3, Min: 1, Max: 6}) {
		t.Fatalf("list = %+v %v", list, err)
	}
	bad := map[string]func(p *domain.Pool){
		"name of up to 80":       func(p *domain.Pool) { p.Name = " " },
		"levels run from 1":      func(p *domain.Pool) { p.LevelMin = 5; p.LevelMax = 2 },
		"low, moderate or high":  func(p *domain.Pool) { p.Difficulty = "deadly" },
		"1 to 20 creatures":      func(p *domain.Pool) { p.Members = nil },
		"weights run from 1":     func(p *domain.Pool) { p.Members[0].Min = 7 },
		"there is no monster":    func(p *domain.Pool) { p.Members[0].Slug = "dragon" },
		"levels run from 1 to 2": func(p *domain.Pool) { p.LevelMax = 21 },
	}
	for want, change := range bad {
		x := goblins()
		change(&x)
		_, err := s.SavePool(ctx, dm, w.campaign, x)
		refused(t, err, want)
	}
	if _, err := s.SavePool(ctx, dm, w.campaign, domain.Pool{ID: domain.PoolID(uuid.New()), Name: "Ghost"}); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update unknown = %v", err)
	}
	if _, err := s.Pools(ctx, player, w.campaign); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("player list = %v", err)
	}
	if err := s.DeletePool(ctx, dm, w.campaign, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePool(ctx, dm, w.campaign, p.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("delete twice = %v", err)
	}
	revs, err := s.PoolRevisions(ctx, dm, w.campaign, p.ID)
	if err != nil || len(revs) != 3 || revs[0].Action != campaigndomain.ActionDelete || revs[2].Action != campaigndomain.ActionCreate || revs[0].Author != "Joris" {
		t.Fatalf("revisions = %+v %v", revs, err)
	}
	back, err := s.RestorePool(ctx, dm, w.campaign, p.ID, 1)
	if err != nil || back.LevelMax != 4 || back.Difficulty != "moderate" || len(back.Members) != 2 {
		t.Fatalf("restore = %+v %v", back, err)
	}
	if revs, _ := s.PoolRevisions(ctx, dm, w.campaign, p.ID); revs[0].Action != campaigndomain.ActionRestore || revs[0].RestoredFrom != 1 {
		t.Fatalf("restore revision = %+v", revs[0])
	}
	if _, err := s.RestorePool(ctx, dm, w.campaign, p.ID, 9); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("restore unknown revision = %v", err)
	}
	if _, err := s.PoolRevisions(ctx, dm, w.campaign, domain.PoolID(uuid.New())); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("revisions of nothing = %v", err)
	}
	for _, op := range []func() error{
		func() error { _, err := s.PoolRevisions(ctx, player, w.campaign, p.ID); return err },
		func() error { _, err := s.RestorePool(ctx, player, w.campaign, p.ID, 1); return err },
		func() error { return s.DeletePool(ctx, player, w.campaign, p.ID) },
	} {
		if err := op(); !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("player = %v", err)
		}
	}
}

func TestTablesHoldEncountersPoolDrawsAndNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	s := service(w, pgstore.New(w.pool))
	pool, _ := s.SavePool(ctx, dm, w.campaign, goblins())
	table := func() domain.Table {
		return domain.Table{Name: "Forest road", RegionID: &w.town, ChancePct: 25, Visibility: domain.Open, Entries: []domain.Entry{
			{Weight: 2, Kind: domain.EntryEncounter, Label: " Ambush ", Monsters: []domain.EntryMonster{{Slug: "goblin", Count: 3}}, PoolID: &pool.ID},
			{Weight: 1, Kind: domain.EntryPool, PoolID: &pool.ID, Monsters: []domain.EntryMonster{{Slug: "ogre", Count: 1}}},
			{Weight: 5, Kind: domain.EntryNothing, Label: "Birdsong", PoolID: &pool.ID},
		}}
	}
	tb, err := s.SaveTable(ctx, dm, w.campaign, table())
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.Tables(ctx, dm, w.campaign)
	e := list[0].Entries
	if err != nil || len(list) != 1 || *list[0].RegionID != w.town || len(e) != 3 || e[0].Label != "Ambush" || e[0].PoolID != nil || len(e[0].Monsters) != 1 ||
		*e[1].PoolID != pool.ID || e[1].Monsters != nil || e[2].PoolID != nil {
		t.Fatalf("tables = %+v %v", list, err)
	}
	stranger := uuid.New()
	missing := domain.PoolID(uuid.New())
	bad := map[string]func(t *domain.Table){
		"chance runs from 0":     func(t *domain.Table) { t.ChancePct = 101 },
		"secret or open":         func(t *domain.Table) { t.Visibility = "loud" },
		"1 to 50 entries":        func(t *domain.Table) { t.Entries = nil },
		"choose a location":      func(t *domain.Table) { t.RegionID = &stranger },
		"one of the campaign's":  func(t *domain.Table) { t.Entries[1].PoolID = &missing },
		"has a name and 1 to 10": func(t *domain.Table) { t.Entries[0].Label = "" },
		"1 to 20 of each":        func(t *domain.Table) { t.Entries[0].Monsters[0].Count = 0 },
		"no monster":             func(t *domain.Table) { t.Entries[0].Monsters[0].Slug = "dragon" },
		"encounter, a pool draw": func(t *domain.Table) { t.Entries[2].Kind = "treasure" },
		"weights run from 1":     func(t *domain.Table) { t.Entries[2].Weight = 0 },
		"name of up to 80":       func(t *domain.Table) { t.Name = "" },
	}
	for want, change := range bad {
		x := table()
		change(&x)
		_, err := s.SaveTable(ctx, dm, w.campaign, x)
		refused(t, err, want)
	}
	refused(t, s.DeletePool(ctx, dm, w.campaign, pool.ID), "a table still draws")
	tb.ChancePct, tb.RegionID = 50, nil
	if _, err := s.SaveTable(ctx, dm, w.campaign, tb); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTable(ctx, dm, w.campaign, domain.Table{ID: domain.TableID(uuid.New())}); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update unknown = %v", err)
	}
	if err := s.DeleteTable(ctx, dm, w.campaign, tb.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTable(ctx, dm, w.campaign, tb.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("delete twice = %v", err)
	}
	back, err := s.RestoreTable(ctx, dm, w.campaign, tb.ID, 1)
	if err != nil || back.ChancePct != 25 || *back.RegionID != w.town || len(back.Entries) != 3 || back.Entries[0].Monsters[0].Count != 3 {
		t.Fatalf("restore = %+v %v", back, err)
	}
	revs, err := s.TableRevisions(ctx, dm, w.campaign, tb.ID)
	if err != nil || len(revs) != 4 {
		t.Fatalf("revisions = %+v %v", revs, err)
	}
	if _, err := s.TableRevisions(ctx, dm, w.campaign, domain.TableID(uuid.New())); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("revisions of nothing = %v", err)
	}
	if _, err := s.RestoreTable(ctx, dm, w.campaign, tb.ID, 9); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("restore unknown = %v", err)
	}
	locs, err := s.Locations(ctx, dm, w.campaign)
	if err != nil || len(locs) != 1 || locs[0].Name != "Oakford" || locs[0].MapName != "Realm" {
		t.Fatalf("locations = %+v %v", locs, err)
	}
	checks, err := s.Checks(ctx, dm, w.campaign)
	if err != nil || len(checks) != 0 {
		t.Fatalf("checks = %+v %v", checks, err)
	}
	for _, op := range []func() error{
		func() error { _, err := s.Tables(ctx, player, w.campaign); return err },
		func() error { _, err := s.Locations(ctx, player, w.campaign); return err },
		func() error { _, err := s.Checks(ctx, player, w.campaign); return err },
		func() error { _, err := s.TableRevisions(ctx, player, w.campaign, tb.ID); return err },
		func() error { _, err := s.SaveTable(ctx, player, w.campaign, table()); return err },
	} {
		if err := op(); !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("player = %v", err)
		}
	}
}

func TestEveryPrepDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	base := service(w, pgstore.New(w.pool))
	pool, _ := base.SavePool(ctx, dm, w.campaign, goblins())
	tb, err := base.SaveTable(ctx, dm, w.campaign, domain.Table{Name: "Road", RegionID: &w.town, ChancePct: 10, Visibility: domain.Secret, Entries: []domain.Entry{
		{Weight: 1, Kind: domain.EntryEncounter, Label: "Pack", Monsters: []domain.EntryMonster{{Slug: "goblin", Count: 2}}},
		{Weight: 1, Kind: domain.EntryPool, PoolID: &pool.ID},
	}})
	if err != nil {
		t.Fatal(err)
	}
	spare, _ := base.SavePool(ctx, dm, w.campaign, goblins())
	if err := base.DeletePool(ctx, dm, w.campaign, spare.ID); err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(s *app.Service) error{
		"pools":     func(s *app.Service) error { _, err := s.Pools(ctx, dm, w.campaign); return err },
		"save pool": func(s *app.Service) error { _, err := s.SavePool(ctx, dm, w.campaign, pool); return err },
		"delete pool": func(s *app.Service) error {
			fresh, err := base.SavePool(ctx, dm, w.campaign, goblins())
			if err != nil {
				t.Fatal(err)
			}
			return s.DeletePool(ctx, dm, w.campaign, fresh.ID)
		},
		"pool revs":    func(s *app.Service) error { _, err := s.PoolRevisions(ctx, dm, w.campaign, pool.ID); return err },
		"restore pool": func(s *app.Service) error { _, err := s.RestorePool(ctx, dm, w.campaign, spare.ID, 1); return err },
		"tables":       func(s *app.Service) error { _, err := s.Tables(ctx, dm, w.campaign); return err },
		"save table":   func(s *app.Service) error { _, err := s.SaveTable(ctx, dm, w.campaign, tb); return err },
		"delete table": func(s *app.Service) error {
			fresh := tb
			fresh.ID = domain.TableID{}
			if fresh, err = base.SaveTable(ctx, dm, w.campaign, fresh); err != nil {
				t.Fatal(err)
			}
			return s.DeleteTable(ctx, dm, w.campaign, fresh.ID)
		},
		"table revs":    func(s *app.Service) error { _, err := s.TableRevisions(ctx, dm, w.campaign, tb.ID); return err },
		"restore table": func(s *app.Service) error { _, err := s.RestoreTable(ctx, dm, w.campaign, tb.ID, 1); return err },
		"locations":     func(s *app.Service) error { _, err := s.Locations(ctx, dm, w.campaign); return err },
		"checks":        func(s *app.Service) error { _, err := s.Checks(ctx, dm, w.campaign); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(service(w, pgstore.NewFaulty(w.pool, f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

func TestLootTablesNestWithoutLoopsAndKeepTheirHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	s := service(w, pgstore.New(w.pool))
	coins, err := s.SaveLootTable(ctx, dm, w.campaign, domain.LootTable{Name: " Purse ", Rolls: 1, Entries: []domain.LootEntry{
		{Weight: 2, Kind: "currency", Coin: "gp", Amount: "2d6x10", Item: "rope"},
		{Weight: 1, Kind: "nothing", Amount: "9", Coin: "cp"},
	}})
	if err != nil || coins.Name != "Purse" || coins.Entries[0].Item != "" || coins.Entries[1].Amount != "" || coins.Entries[1].Coin != "" {
		t.Fatalf("purse = %+v %v", coins, err)
	}
	hoard, err := s.SaveLootTable(ctx, dm, w.campaign, domain.LootTable{Name: "Hoard", Rolls: 3, Entries: []domain.LootEntry{
		{Weight: 1, Kind: "table", Table: &coins.ID}, {Weight: 1, Kind: "item", Item: "rope", Amount: "1d4"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	missing := domain.LootTableID(uuid.New())
	bad := map[string]func(t *domain.LootTable){
		"name of up to 80":   func(t *domain.LootTable) { t.Name = "" },
		"rolled 1 to 10":     func(t *domain.LootTable) { t.Rolls = 11 },
		"1 to 50 entries":    func(t *domain.LootTable) { t.Entries = nil },
		"weights run from 1": func(t *domain.LootTable) { t.Entries[0].Weight = 0 },
		"write amounts as":   func(t *domain.LootTable) { t.Entries[1].Amount = "lots" },
		"there is no item":   func(t *domain.LootTable) { t.Entries[1].Item = "sword-of-ages" },
		"coins are cp": func(t *domain.LootTable) {
			t.Entries[1] = domain.LootEntry{Weight: 1, Kind: "currency", Coin: "dollar", Amount: "1"}
		},
		"one of the campaign's":   func(t *domain.LootTable) { t.Entries[0].Table = &missing },
		"an item, coins, another": func(t *domain.LootTable) { t.Entries[1].Kind = "spell" },
		"cannot roll on itself":   func(t *domain.LootTable) { t.Entries[0].Table = &t.ID },
	}
	for want, change := range bad {
		x := hoard
		x.Entries = append([]domain.LootEntry(nil), hoard.Entries...)
		change(&x)
		_, err := s.SaveLootTable(ctx, dm, w.campaign, x)
		refused(t, err, want)
	}
	loop := coins
	loop.Entries = []domain.LootEntry{{Weight: 1, Kind: "table", Table: &hoard.ID}}
	_, err = s.SaveLootTable(ctx, dm, w.campaign, loop)
	refused(t, err, "cannot roll on itself")
	refused(t, s.DeleteLootTable(ctx, dm, w.campaign, coins.ID), "still rolls on this one")
	if _, err := s.SaveLootTable(ctx, dm, w.campaign, domain.LootTable{ID: missing, Name: "Ghost"}); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update unknown = %v", err)
	}
	hoard.Rolls = 1
	if _, err := s.SaveLootTable(ctx, dm, w.campaign, hoard); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLootTable(ctx, dm, w.campaign, hoard.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLootTable(ctx, dm, w.campaign, hoard.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("delete twice = %v", err)
	}
	back, err := s.RestoreLootTable(ctx, dm, w.campaign, hoard.ID, 1)
	if err != nil || back.Rolls != 3 || len(back.Entries) != 2 || *back.Entries[0].Table != coins.ID || back.Entries[1].Amount != "1d4" {
		t.Fatalf("restore = %+v %v", back, err)
	}
	revs, err := s.LootTableRevisions(ctx, dm, w.campaign, hoard.ID)
	if err != nil || len(revs) != 4 || revs[0].RestoredFrom != 1 {
		t.Fatalf("revisions = %+v %v", revs, err)
	}
	if _, err := s.LootTableRevisions(ctx, dm, w.campaign, missing); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("revisions of nothing = %v", err)
	}
	if _, err := s.RestoreLootTable(ctx, dm, w.campaign, hoard.ID, 9); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("restore unknown = %v", err)
	}
	list, err := s.LootTables(ctx, dm, w.campaign)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v %v", list, err)
	}
	for _, op := range []func() error{
		func() error { _, err := s.LootTables(ctx, player, w.campaign); return err },
		func() error { _, err := s.LootTableRevisions(ctx, player, w.campaign, hoard.ID); return err },
		func() error { return s.DeleteLootTable(ctx, player, w.campaign, hoard.ID) },
	} {
		if err := op(); !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("player = %v", err)
		}
	}
	ops := map[string]func(s *app.Service) error{
		"loot tables": func(s *app.Service) error { _, err := s.LootTables(ctx, dm, w.campaign); return err },
		"save loot":   func(s *app.Service) error { _, err := s.SaveLootTable(ctx, dm, w.campaign, back); return err },
		"delete loot": func(s *app.Service) error {
			fresh := back
			fresh.ID = domain.LootTableID{}
			if fresh, err = service(w, pgstore.New(w.pool)).SaveLootTable(ctx, dm, w.campaign, fresh); err != nil {
				t.Fatal(err)
			}
			return s.DeleteLootTable(ctx, dm, w.campaign, fresh.ID)
		},
		"loot revs":    func(s *app.Service) error { _, err := s.LootTableRevisions(ctx, dm, w.campaign, hoard.ID); return err },
		"restore loot": func(s *app.Service) error { _, err := s.RestoreLootTable(ctx, dm, w.campaign, hoard.ID, 1); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(service(w, pgstore.NewFaulty(w.pool, f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

func TestSettlementsAndShopsStockFromLootAndKeepTheirHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	s := service(w, pgstore.New(w.pool))
	var npc uuid.UUID
	if err := w.pool.QueryRow(ctx, "INSERT INTO campaign.npcs (campaign_id, name) VALUES ($1, 'Hilda') RETURNING id", w.campaign).Scan(&npc); err != nil {
		t.Fatal(err)
	}
	wares, err := s.SaveLootTable(ctx, dm, w.campaign, domain.LootTable{Name: "Wares", Rolls: 1, Entries: []domain.LootEntry{
		{Weight: 1, Kind: "item", Item: "rope", Amount: "2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	treasure, _ := s.SaveLootTable(ctx, dm, w.campaign, domain.LootTable{Name: "Treasure", Rolls: 3, Entries: []domain.LootEntry{
		{Weight: 1, Kind: "item", Item: "crown", Amount: "1"}, {Weight: 1, Kind: "item", Item: "wand", Amount: "1"}, {Weight: 1, Kind: "currency", Coin: "gp", Amount: "5"},
	}})
	town, err := s.SaveSettlement(ctx, dm, w.campaign, domain.Settlement{Name: " Oakford ", Size: "town", Wealth: "modest", LocationID: &w.town})
	if err != nil || town.Name != "Oakford" {
		t.Fatalf("settlement = %+v %v", town, err)
	}
	stranger := uuid.New()
	for want, x := range map[string]domain.Settlement{
		"name of up to 80": {Size: "town", Wealth: "modest"},
		"hamlet, village":  {Name: "A", Size: "megacity", Wealth: "modest"},
		"poor, modest":     {Name: "A", Size: "town", Wealth: "rich"},
		"location on one":  {Name: "A", Size: "town", Wealth: "modest", LocationID: &stranger},
	} {
		_, err := s.SaveSettlement(ctx, dm, w.campaign, x)
		refused(t, err, want)
	}
	// The Shop belongs to a Faction of the Campaign.
	watch := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
		VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', 0, now(), now())`, watch, w.campaign); err != nil {
		t.Fatal(err)
	}
	shop := domain.Shop{SettlementID: town.ID, Name: "Store", Kind: " general ", OwnerID: &npc, FactionID: &watch, MarkupPct: 50, HaggleDC: 15, HagglePct: 20, LootTable: &wares.ID, Restock: "days", RestockDays: 3}
	store, err := s.SaveShop(ctx, dm, w.campaign, shop)
	if err != nil || store.Kind != "general" || len(store.Stock) != 0 || store.FactionID == nil || *store.FactionID != watch {
		t.Fatalf("shop = %+v %v", store, err)
	}
	if listed, err := s.Shops(ctx, dm, w.campaign); err != nil || len(listed) != 1 || listed[0].FactionID == nil || *listed[0].FactionID != watch {
		t.Fatalf("the listed shop = %+v %v", listed, err)
	}
	// An entry of an Encounter Table can be the Faction's own; a restored Table keeps whose it was.
	patrols, err := s.SaveTable(ctx, dm, w.campaign, domain.Table{Name: "Watch road", ChancePct: 10, Visibility: domain.Secret, Entries: []domain.Entry{
		{Weight: 2, Kind: domain.EntryNothing, Label: "A patrol passes", FactionID: &watch}, {Weight: 1, Kind: domain.EntryNothing, Label: "Quiet"},
	}})
	if err != nil || patrols.Entries[0].FactionID == nil || *patrols.Entries[0].FactionID != watch || patrols.Entries[1].FactionID != nil {
		t.Fatalf("a table with a Faction's entry = %+v %v", patrols, err)
	}
	if listed, err := s.Tables(ctx, dm, w.campaign); err != nil || !slices.ContainsFunc(listed, func(x domain.Table) bool {
		return x.ID == patrols.ID && x.Entries[0].FactionID != nil && *x.Entries[0].FactionID == watch
	}) {
		t.Fatalf("the listed table = %+v %v", listed, err)
	}
	if err := s.DeleteTable(ctx, dm, w.campaign, patrols.ID); err != nil {
		t.Fatal(err)
	}
	if back, err := s.RestoreTable(ctx, dm, w.campaign, patrols.ID, 1); err != nil || back.Entries[0].FactionID == nil || *back.Entries[0].FactionID != watch {
		t.Fatalf("the restored table = %+v %v", back, err)
	}
	_, err = s.SaveTable(ctx, dm, w.campaign, domain.Table{Name: "Nowhere", ChancePct: 10, Visibility: domain.Secret, Entries: []domain.Entry{{Weight: 1, Kind: domain.EntryNothing, FactionID: &stranger}}})
	refused(t, err, "campaign's Factions")
	missingTable := domain.LootTableID(uuid.New())
	for want, change := range map[string]func(x *domain.Shop){
		"trade of up to 40":      func(x *domain.Shop) { x.Kind = "" },
		"markup runs":            func(x *domain.Shop) { x.MarkupPct = 301 },
		"haggling has a DC":      func(x *domain.Shop) { x.HaggleDC = 40 },
		"restocks never":         func(x *domain.Shop) { x.Restock = "weekly" },
		"every 1 to 365":         func(x *domain.Shop) { x.RestockDays = 0 },
		"campaign's settlements": func(x *domain.Shop) { x.SettlementID = domain.SettlementID(uuid.New()) },
		"NPCs as the owner":      func(x *domain.Shop) { x.OwnerID = &stranger },
		"campaign's Factions":    func(x *domain.Shop) { x.FactionID = &stranger },
		"loot tables for":        func(x *domain.Shop) { x.LootTable = &missingTable },
		"name of up to 80":       func(x *domain.Shop) { x.Name = "" },
	} {
		x := shop
		change(&x)
		_, err := s.SaveShop(ctx, dm, w.campaign, x)
		refused(t, err, want)
	}
	stocked, err := s.RerollStock(ctx, dm, w.campaign, store.ID)
	if err != nil || len(stocked.Stock) != 1 || stocked.Stock[0] != (domain.StockItem{Slug: "rope", Quantity: 6, PriceCP: 150}) {
		t.Fatalf("a town rolls the wares three times = %+v %v", stocked, err)
	}
	store.LootTable, store.Restock = &treasure.ID, "never"
	if store, err = s.SaveShop(ctx, dm, w.campaign, store); err != nil || store.RestockDays != 0 || len(store.Stock) != 1 {
		t.Fatalf("an edit keeps the stock = %+v %v", store, err)
	}
	rich, err := s.RerollStock(ctx, dm, w.campaign, store.ID)
	if err != nil || slices.ContainsFunc(rich.Stock, func(k domain.StockItem) bool { return k.Slug == "crown" || k.Slug == "wand" }) {
		t.Fatalf("a modest town stocks nothing dearer than 100 gp = %+v %v", rich, err)
	}
	town.Wealth = "wealthy"
	if _, err := s.SaveSettlement(ctx, dm, w.campaign, town); err != nil {
		t.Fatal(err)
	}
	rich, _ = s.RerollStock(ctx, dm, w.campaign, store.ID)
	if !slices.ContainsFunc(rich.Stock, func(k domain.StockItem) bool { return k.Slug == "wand" && k.PriceCP == 60000 }) {
		t.Fatalf("a wealthy town prices an uncommon wand at 400 gp plus markup = %+v", rich.Stock)
	}
	refused(t, s.DeleteSettlement(ctx, dm, w.campaign, town.ID), "shops first")
	if err := s.DeleteShop(ctx, dm, w.campaign, store.ID); err != nil {
		t.Fatal(err)
	}
	back, err := s.RestoreShop(ctx, dm, w.campaign, store.ID, 2)
	if err != nil || len(back.Stock) != 1 || back.Stock[0].Quantity != 6 || back.Restock != "days" || back.FactionID == nil || *back.FactionID != watch {
		t.Fatalf("restoring the shop brings its stock back = %+v %v", back, err)
	}
	revs, err := s.ShopRevisions(ctx, dm, w.campaign, store.ID)
	if err != nil || len(revs) != 7 {
		t.Fatalf("shop revisions = %d %v", len(revs), err)
	}
	if err := s.DeleteShop(ctx, dm, w.campaign, store.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSettlement(ctx, dm, w.campaign, town.ID); err != nil {
		t.Fatal(err)
	}
	if again, err := s.RestoreSettlement(ctx, dm, w.campaign, town.ID, 1); err != nil || again.Wealth != "modest" {
		t.Fatalf("restore settlement = %+v %v", again, err)
	}
	if revs, err := s.SettlementRevisions(ctx, dm, w.campaign, town.ID); err != nil || len(revs) != 4 {
		t.Fatalf("settlement revisions = %d %v", len(revs), err)
	}
	for name, err := range map[string]error{
		"settlement unknown": func() error {
			_, err := s.SaveSettlement(ctx, dm, w.campaign, domain.Settlement{ID: domain.SettlementID(uuid.New())})
			return err
		}(),
		"shop unknown": func() error {
			_, err := s.SaveShop(ctx, dm, w.campaign, domain.Shop{ID: domain.ShopID(uuid.New())})
			return err
		}(),
		"delete shop":       s.DeleteShop(ctx, dm, w.campaign, domain.ShopID(uuid.New())),
		"delete settlement": s.DeleteSettlement(ctx, dm, w.campaign, domain.SettlementID(uuid.New())),
		"reroll unknown":    func() error { _, err := s.RerollStock(ctx, dm, w.campaign, domain.ShopID(uuid.New())); return err }(),
		"revisions":         func() error { _, err := s.ShopRevisions(ctx, dm, w.campaign, domain.ShopID(uuid.New())); return err }(),
	} {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s = %v", name, err)
		}
	}
	if _, err := s.Shops(ctx, player, w.campaign); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("player = %v", err)
	}
	if _, err := s.Settlements(ctx, player, w.campaign); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("player = %v", err)
	}
	shop.LootTable = &wares.ID
	live, _ := s.SaveShop(ctx, dm, w.campaign, shop)
	ops := map[string]func(x *app.Service) error{
		"settlements":  func(x *app.Service) error { _, err := x.Settlements(ctx, dm, w.campaign); return err },
		"save town":    func(x *app.Service) error { _, err := x.SaveSettlement(ctx, dm, w.campaign, town); return err },
		"shops":        func(x *app.Service) error { _, err := x.Shops(ctx, dm, w.campaign); return err },
		"save shop":    func(x *app.Service) error { _, err := x.SaveShop(ctx, dm, w.campaign, live); return err },
		"reroll":       func(x *app.Service) error { _, err := x.RerollStock(ctx, dm, w.campaign, live.ID); return err },
		"restore":      func(x *app.Service) error { _, err := x.RestoreShop(ctx, dm, w.campaign, store.ID, 2); return err },
		"restore town": func(x *app.Service) error { _, err := x.RestoreSettlement(ctx, dm, w.campaign, town.ID, 1); return err },
		"delete shop": func(x *app.Service) error {
			fresh := shop
			fresh, err := s.SaveShop(ctx, dm, w.campaign, fresh)
			if err != nil {
				t.Fatal(err)
			}
			return x.DeleteShop(ctx, dm, w.campaign, fresh.ID)
		},
		"delete town": func(x *app.Service) error {
			fresh, err := s.SaveSettlement(ctx, dm, w.campaign, domain.Settlement{Name: "Ghost", Size: "hamlet", Wealth: "poor"})
			if err != nil {
				t.Fatal(err)
			}
			return x.DeleteSettlement(ctx, dm, w.campaign, fresh.ID)
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(service(w, pgstore.NewFaulty(w.pool, f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
