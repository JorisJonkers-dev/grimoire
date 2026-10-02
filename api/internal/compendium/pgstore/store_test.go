package pgstore_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

func feet(n int) *int { return &n }

func fixture() snapshot.Snapshot {
	return snapshot.Snapshot{
		Documents: []snapshot.Document{
			{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "new", URL: "https://a"},
			{Key: "srd-2014", Title: "SRD 5.1", RulesetYear: 2014, Precedence: 10, License: "CC-BY-4.0", Attribution: "old", URL: "https://b"},
		},
		Spells: []snapshot.Spell{
			{
				Document: "srd-2024", Slug: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "evocation", CastingTime: "action",
				RangeText: "120 feet", RangeFeet: feet(120), Verbal: true, Somatic: true, Duration: "instantaneous",
				Description: "The target is knocked prone.", Classes: []string{"sorcerer", "wizard"}, AttackRoll: true,
				DamageRoll: "1d10", DamageTypes: []string{"fire"}, Scaling: []snapshot.Scaling{{Kind: "character", Level: 5, DamageRoll: "2d10"}},
			},
			{
				Document: "srd-2014", Slug: "fire-bolt", Name: "Fire Bolt", Level: 0, School: "evocation", CastingTime: "1 action",
				RangeText: "120 feet", Duration: "instantaneous", Description: "Old text.", Classes: []string{"wizard"},
				DamageTypes: []string{"fire"}, Scaling: []snapshot.Scaling{},
			},
			{
				Document: "srd-2014", Slug: "hold-person", Name: "Hold Person", Level: 2, School: "enchantment", CastingTime: "1 action",
				RangeText: "60 feet", Material: true, MaterialText: "iron", Concentration: true, Duration: "1 minute",
				Description: "Paralyzed.", HigherLevel: "More targets.", Classes: []string{"cleric"}, SaveAbility: "wisdom",
				DamageTypes: []string{}, Scaling: []snapshot.Scaling{},
			},
			{
				Document: "srd-2024", Slug: "alarm", Name: "Alarm", Level: 1, School: "abjuration", CastingTime: "1 minute",
				RangeText: "30 feet", Ritual: true, Duration: "8 hours", Description: "A bell.", Classes: []string{"wizard"},
				DamageTypes: []string{}, Scaling: []snapshot.Scaling{},
			},
		},
		Conditions: []snapshot.Condition{
			{Document: "srd-2024", Slug: "prone", Name: "Prone", Description: "On the ground."},
			{Document: "srd-2014", Slug: "paralyzed", Name: "Paralyzed", Description: "Frozen."},
		},
	}
}

func newStore(t *testing.T) *pgstore.Store {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return pgstore.New(store.Pool())
}

func TestImportIsIdempotentAndVersioned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStore(t)
	if v, _ := s.Version(ctx); v != 0 {
		t.Fatalf("empty version = %d", v)
	}
	changed, err := s.Import(ctx, fixture(), "h1")
	if err != nil || !changed {
		t.Fatalf("first import: %v %v", changed, err)
	}
	if changed, err := s.Import(ctx, fixture(), "h1"); err != nil || changed {
		t.Fatalf("same snapshot re-imported: %v %v", changed, err)
	}
	if changed, err := s.Import(ctx, fixture(), "h2"); err != nil || !changed {
		t.Fatalf("new snapshot: %v %v", changed, err)
	}
	if v, _ := s.Version(ctx); v != 2 {
		t.Fatalf("version = %d", v)
	}
	sources, err := s.ListSources(ctx)
	if err != nil || len(sources) != 2 || sources[0].Key != "srd-2024" || sources[1].Attribution != "old" {
		t.Fatalf("sources = %+v %v", sources, err)
	}
}

func TestListSpellsBlendsAndFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.Import(ctx, fixture(), "h"); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListSpells(ctx, compendium.SpellFilter{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Slug != "alarm" || all[1].Slug != "fire-bolt" || all[1].Ruleset != "srd-2024" {
		t.Fatalf("blend = %+v", all)
	}
	level := 2
	cases := map[string]compendium.SpellFilter{
		"query":   {Query: "BOLT", PageSize: 10},
		"level":   {Level: &level, PageSize: 10},
		"school":  {School: "abjuration", PageSize: 10},
		"class":   {Class: "cleric", PageSize: 10},
		"ruleset": {Ruleset: "srd-2014", Query: "fire", PageSize: 10},
		"after":   {After: &compendium.Cursor{Name: "Fire Bolt", Slug: "fire-bolt"}, PageSize: 10},
		"page":    {PageSize: 1},
	}
	want := map[string]string{"query": "fire-bolt", "level": "hold-person", "school": "alarm", "class": "hold-person", "ruleset": "fire-bolt", "after": "hold-person", "page": "alarm"}
	for name, f := range cases {
		got, err := s.ListSpells(ctx, f)
		if err != nil || len(got) != 1 || got[0].Slug != want[name] {
			t.Errorf("%s: %+v %v", name, got, err)
		}
	}
	old, _ := s.ListSpells(ctx, compendium.SpellFilter{Ruleset: "srd-2014", Query: "fire", PageSize: 10})
	if old[0].Ruleset != "srd-2014" {
		t.Fatalf("ruleset filter ignored: %+v", old)
	}
}

func TestGetSpellWithChildrenAndMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.Import(ctx, fixture(), "h"); err != nil {
		t.Fatal(err)
	}
	bolt, err := s.GetSpell(ctx, "fire-bolt", "")
	if err != nil {
		t.Fatal(err)
	}
	if bolt.Ruleset != "srd-2024" || *bolt.RangeFeet != 120 || len(bolt.Classes) != 2 || bolt.DamageTypes[0] != "fire" ||
		len(bolt.Scaling) != 1 || len(bolt.Mentions) != 1 || bolt.Mentions[0].Slug != "prone" {
		t.Fatalf("fire bolt = %+v", bolt)
	}
	old, err := s.GetSpell(ctx, "fire-bolt", "srd-2014")
	if err != nil || old.Description != "Old text." || old.RangeFeet != nil {
		t.Fatalf("2014 fire bolt = %+v %v", old, err)
	}
	hold, err := s.GetSpell(ctx, "hold-person", "")
	if err != nil || hold.SaveAbility != "wisdom" || hold.MaterialText != "iron" || hold.HigherLevel != "More targets." || hold.Mentions[0].Name != "Paralyzed" {
		t.Fatalf("hold person = %+v %v", hold, err)
	}
	if _, err := s.GetSpell(ctx, "alarm", "srd-2014"); !errors.Is(err, compendium.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
}

func TestImportRejectsUnknownAbility(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStore(t)
	bad := fixture()
	bad.Spells[2].SaveAbility = "luck"
	if _, err := s.Import(ctx, bad, "bad"); err == nil {
		t.Fatal("unknown ability accepted")
	}
	if v, _ := s.Version(ctx); v != 0 {
		t.Fatal("failed import must roll back")
	}
}

func TestStoreErrorsWhenDatabaseIsGone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	s := pgstore.New(store.Pool())
	store.Close()
	if _, err := s.Import(ctx, fixture(), "h"); err == nil {
		t.Error("import")
	}
	if _, err := s.ListSources(ctx); err == nil {
		t.Error("sources")
	}
	if _, err := s.ListSpells(ctx, compendium.SpellFilter{PageSize: 1}); err == nil {
		t.Error("list")
	}
	if _, err := s.GetSpell(ctx, "alarm", ""); err == nil {
		t.Error("get")
	}
}

func TestEmbeddedSnapshotImports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sub, err := fs.Sub(db.Seeds, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	snap, hash, err := snapshot.Load(sub)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(t)
	if _, err := s.Import(ctx, snap, hash); err != nil {
		t.Fatal(err)
	}
	fireball, err := s.GetSpell(ctx, "fireball", "")
	if err != nil || fireball.Ruleset != "srd-2024" || fireball.Level != 3 || len(fireball.Scaling) == 0 {
		t.Fatalf("fireball = %+v %v", fireball.SpellSummary, err)
	}
	assertBuilderOptions(t, s)
	assertTraits(t, s)
}

func assertTraits(t *testing.T, s *pgstore.Store) {
	t.Helper()
	ctx := context.Background()
	traits, err := s.Traits(ctx, "srd-2024", "dwarf", []compendium.ClassLevel{{Class: "fighter", Subclass: "champion", Level: 3}}, []string{"alert"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]compendium.Trait{}
	for _, tr := range traits {
		got[tr.Name] = tr
	}
	if got["Second Wind"].Level != 1 || got["Action Surge"].Level != 2 || got["Darkvision"].Source != "species" {
		t.Fatalf("traits = %+v", traits)
	}
	if _, ok := got["Indomitable"]; ok {
		t.Fatal("a level 9 feature at level 3")
	}
	if got["Improved Critical"].Level != 3 || got["Alert"].Source != "feat" {
		t.Fatalf("subclass and feat traits = %+v", traits)
	}
	lu, err := s.LevelUpOptions(ctx, "srd-2024", "wizard", 1)
	if err != nil || len(lu.Subclasses) != 1 || lu.Subclasses[0].Slug != "evoker" || len(lu.Feats) != 17 {
		t.Fatalf("wizard options = %+v %v", lu.Subclasses, err)
	}
	levels := map[int]bool{}
	for _, sp := range lu.Spells {
		levels[sp.Level] = true
	}
	if !levels[0] || !levels[1] || levels[2] {
		t.Fatalf("wizard spells to level 1 = %v", levels)
	}
	cat, err := s.Features(ctx)
	if err != nil || cat.MasteryCount("fighter", 1) != 3 || cat.MasteryCount("fighter", 4) != 4 || cat.MasteryCount("wizard", 5) != 0 {
		t.Fatalf("mastery counts: %v", err)
	}
}

func assertBuilderOptions(t *testing.T, s *pgstore.Store) {
	t.Helper()
	ctx := context.Background()
	o, err := s.BuilderOptions(ctx, "srd-2024")
	if err != nil || o.RulesetYear != 2024 || len(o.Classes) != 12 || len(o.Backgrounds) != 4 || len(o.Weapons) == 0 {
		t.Fatalf("2024 options = %d classes %d backgrounds %v", len(o.Classes), len(o.Backgrounds), err)
	}
	acolyte := o.Backgrounds[0]
	if acolyte.Slug != "acolyte" || strings.Join(acolyte.Abilities, ",") != "intelligence,wisdom,charisma" ||
		strings.Join(acolyte.Skills, ",") != "insight,religion" {
		t.Fatalf("acolyte = %+v", acolyte)
	}
	shield := false
	for _, a := range o.Armor {
		shield = shield || (a.Shield && a.ACBase == 2)
	}
	if !shield {
		t.Fatalf("no shield in %+v", o.Armor)
	}
	old, err := s.BuilderOptions(ctx, "srd-2014")
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range old.Species {
		if sp.Slug == "dwarf" && sp.SpeedFeet != 25 {
			t.Fatalf("2014 dwarf speed = %d", sp.SpeedFeet)
		}
	}
	if _, err := s.BuilderOptions(ctx, "homebrew"); !errors.Is(err, compendium.ErrNotFound) {
		t.Fatalf("unknown ruleset: %v", err)
	}
}
