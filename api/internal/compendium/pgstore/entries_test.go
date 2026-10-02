package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

func entryStore(t *testing.T) *pgstore.Store {
	t.Helper()
	s := newStore(t)
	snap := fixture()
	pgstore.AddSampleEntries(&snap)
	if _, err := s.Import(context.Background(), snap, "entries"); err != nil {
		t.Fatal(err)
	}
	return s
}

func facts(e compendium.EntryDetail) map[string]string {
	out := map[string]string{}
	for _, f := range e.Facts {
		out[f.Label] = f.Value
	}
	return out
}

func TestListEntriesBlendsAndPages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := entryStore(t)
	classes, err := s.ListEntries(ctx, compendium.EntryFilter{Kind: "class", PageSize: 10})
	if err != nil || len(classes) != 2 || classes[0].Slug != "champion" || classes[0].Subtitle != "Subclass of Fighter" ||
		classes[1].Ruleset != "srd-2024" {
		t.Fatalf("classes = %+v %v", classes, err)
	}
	cases := map[string]compendium.EntryFilter{
		"ruleset": {Kind: "class", Ruleset: "srd-2014", PageSize: 10},
		"query":   {Kind: "weapon", Query: "LONG", PageSize: 10},
		"after":   {Kind: "armor", After: &compendium.Cursor{Name: "Leather", Slug: "leather"}, PageSize: 10},
		"magic":   {Kind: "magic-item", PageSize: 10},
		"item":    {Kind: "item", PageSize: 10},
		"monster": {Kind: "monster", PageSize: 10},
		"page":    {Kind: "feat", PageSize: 1},
	}
	want := map[string]string{
		"ruleset": "Class", "query": "1d8 · martial", "after": "Heavy armour · AC 18", "magic": "Uncommon", "item": "Gear",
		"monster": "CR 0.25 · Humanoid", "page": "General feat",
	}
	for name, f := range cases {
		got, err := s.ListEntries(ctx, f)
		if err != nil || len(got) != 1 || got[0].Subtitle != want[name] {
			t.Errorf("%s: %+v %v", name, got, err)
		}
	}
}

func TestGetEntryPresentsEachKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := entryStore(t)
	fighter, err := s.GetEntry(ctx, "class", "fighter", "")
	if err != nil || fighter.Ruleset != "srd-2024" || facts(fighter)["Hit Die"] != "d10" || facts(fighter)["Saving Throws"] != "Strength, Constitution" ||
		fighter.Sections[1].Title != "Second Wind (level 1)" || fighter.Sections[2].Title != "Fighter Table" {
		t.Fatalf("fighter = %+v %v", fighter, err)
	}
	champion, _ := s.GetEntry(ctx, "class", "champion", "")
	if facts(champion)["Subclass of"] != "Fighter" || facts(champion)["Spellcasting"] != "Third" || len(champion.Sections) != 0 {
		t.Fatalf("champion = %+v", champion)
	}
	cases := map[string]struct{ kind, slug, label, value string }{
		"feat":      {"feat", "grappler", "Prerequisite", "Level 4+"},
		"bow range": {"weapon", "longbow", "Range", "150/600 ft"},
		"bow props": {"weapon", "longbow", "Properties", "Ammunition (arrow), Heavy"},
		"mastery":   {"weapon", "longbow", "Mastery", "Slow"},
		"club":      {"weapon", "club", "Category", "Simple"},
		"plate":     {"armor", "plate-armor", "Stealth", "Disadvantage"},
		"strength":  {"armor", "plate-armor", "Strength", "15"},
		"half":      {"armor", "half-plate", "Armor Class", "15 + Dex (max 2)"},
		"leather":   {"armor", "leather", "Armor Class", "11 + Dex"},
		"rope":      {"item", "rope", "Weight", "5 lb"},
		"cost":      {"item", "rope", "Cost", "1 gp"},
		"bag":       {"magic-item", "bag-of-holding", "Attunement", "Required by a wizard"},
		"cr":        {"monster", "goblin", "Challenge", "0.25 (50 XP)"},
		"dex":       {"monster", "goblin", "DEX", "14 (+2)"},
		"wis":       {"monster", "goblin", "WIS", "8 (-1)"},
		"speed":     {"monster", "goblin", "Speed", "Walk 30 ft"},
		"skills":    {"monster", "goblin", "Skills", "Stealth +6"},
		"immune":    {"monster", "goblin", "Condition Immunities", "Charmed"},
		"ac":        {"monster", "goblin", "Armor Class", "15 (leather)"},
	}
	for name, c := range cases {
		got, err := s.GetEntry(ctx, c.kind, c.slug, "")
		if err != nil || facts(got)[c.label] != c.value {
			t.Errorf("%s: %q in %+v %v", name, facts(got)[c.label], got.Facts, err)
		}
	}
}

func TestGetEntryFindsMentionsAndBodies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := entryStore(t)
	goblin, _ := s.GetEntry(ctx, "monster", "goblin", "")
	if len(goblin.Mentions) != 1 || goblin.Mentions[0].Slug != "prone" || goblin.Sections[3].Title != "Hide (bonus action)" {
		t.Fatalf("goblin = %+v", goblin)
	}
	if i := slices.IndexFunc(goblin.Sections, func(x compendium.Section) bool { return x.Title == "Visibility" }); i < 0 ||
		goblin.Sections[i].Text != "Sight: a check against Hidden, Disguised, Illusory, Secret; never Invisible, Ethereal, Darkness, Heavy obscurement.\nDarkvision 60 ft: Darkness." {
		t.Fatalf("the goblin's Visibility = %+v", goblin.Sections)
	}
	for kind, slug := range map[string]string{"species": "dwarf", "background": "sage", "condition": "prone"} {
		got, err := s.GetEntry(ctx, kind, slug, "")
		if err != nil || len(got.Sections) == 0 {
			t.Errorf("%s: %+v %v", kind, got, err)
		}
	}
	grappler, _ := s.GetEntry(ctx, "feat", "grappler", "")
	if grappler.Sections[0].Text != "Knock a target prone.\n\nHold on." || grappler.Mentions[0].Slug != "prone" {
		t.Fatalf("grappler = %+v", grappler)
	}
}

func TestGetEntryNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := entryStore(t)
	for _, c := range [][3]string{{"spell", "fire-bolt", ""}, {"class", "wizard", ""}, {"feat", "grappler", "srd-2014"}} {
		if _, err := s.GetEntry(ctx, c[0], c[1], c[2]); !errors.Is(err, compendium.ErrNotFound) {
			t.Errorf("%v: %v", c, err)
		}
	}
}

func TestAutomationCoverageCountsEveryKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := entryStore(t)
	counts, err := s.AutomationCoverage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string]compendium.AutomationCount{}
	for _, c := range counts {
		byKind[c.Kind] = c
	}
	if byKind["class"].Total != 3 || byKind["spell"].Total != 4 || byKind["armor"].Manual != 3 || byKind["monster"].Full != 0 {
		t.Fatalf("coverage = %+v", counts)
	}
	if c := byKind["condition"]; c.Total != 2 || c.Full != 2 || c.Manual != 0 || c.Partial != 0 {
		t.Fatalf("prone and paralysis are both modelled = %+v", c)
	}
}

func TestEntryReadsFailWhenDatabaseIsGone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	s := pgstore.New(store.Pool())
	store.Close()
	if _, err := s.ListEntries(ctx, compendium.EntryFilter{Kind: "class", PageSize: 1}); err == nil {
		t.Error("list")
	}
	if _, err := s.GetEntry(ctx, "class", "fighter", ""); err == nil {
		t.Error("get")
	}
	if _, err := s.AutomationCoverage(ctx); err == nil {
		t.Error("coverage")
	}
}

func TestBuilderOptionsFromSample(t *testing.T) {
	t.Parallel()
	o, err := entryStore(t).BuilderOptions(context.Background(), "srd-2024")
	if err != nil || len(o.Classes) != 1 || o.Classes[0].HitDie != 10 || len(o.Classes[0].Saves) != 2 {
		t.Fatalf("classes = %+v %v", o.Classes, err)
	}
	if o.Species[0].SpeedFeet != 30 || len(o.Backgrounds[0].Skills) != 0 || o.Armor[0].DexCap != -1 || o.Armor[1].DexCap != 2 {
		t.Fatalf("options = %+v", o)
	}
	if o.Weapons[0].Slug != "club" || o.Weapons[1].DamageType != "fire" {
		t.Fatalf("weapons = %+v", o.Weapons)
	}
}
