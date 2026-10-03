package subclassbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
)

var classes = []string{"fighter", "wizard"}

func warden() subclassbuild.Design {
	return subclassbuild.Design{
		Class: "fighter",
		Features: []subclassbuild.Feature{
			{Level: 3, Name: "Lantern Oath", Text: "You carry a lantern that never gutters.", Uses: "", Spell: "dancing-lights", SpellName: "Dancing Lights"},
			{Level: 3, Name: "Kindle", Text: "As a Bonus Action you brighten the lantern.", Uses: "light", Spell: "light", SpellName: "Light"},
			{Level: 7, Name: "Steady Flame", Text: "Your lantern burns through magical darkness.", Uses: "embers", Spell: "", SpellName: ""},
		},
		Resources: []subclassbuild.Resource{
			{Key: "light", Name: "Lantern Light", Basis: subclassbuild.Proficiency, Amount: 0, Ability: "", FromLevel: 3, Die: "", Recharge: "long_rest"},
			{Key: "embers", Name: "Embers", Basis: subclassbuild.Fixed, Amount: 2, Ability: "", FromLevel: 7, Die: "d6", Recharge: "short_rest"},
			{Key: "glow", Name: "Glow", Basis: subclassbuild.ClassLevel, Amount: 2, Ability: "", FromLevel: 3, Die: "", Recharge: "dawn"},
			{Key: "will", Name: "Will", Basis: subclassbuild.Ability, Amount: 0, Ability: "wisdom", FromLevel: 3, Die: "", Recharge: "long_rest"},
		},
		Choices: []subclassbuild.Choice{
			{Level: 3, Name: "Lantern Style", Count: 1, Options: []string{"Bog Glass", "Ember Wick"}},
		},
	}
}

func TestAWardenCompilesIntoFeatureData(t *testing.T) {
	t.Parallel()
	d := warden()
	if err := subclassbuild.Check(d, classes); err != nil {
		t.Fatal(err)
	}
	sc := subclassbuild.Compile("hb-0190c7a80000", "Lantern Warden", d)
	if sc.Slug != "hb-0190c7a80000" || sc.Name != "Lantern Warden" || sc.Class != "fighter" {
		t.Fatalf("subclass = %+v", sc)
	}
	owner := features.Owner{Kind: "subclass", Slug: "hb-0190c7a80000"}
	want := []subclassbuild.Trait{
		{Level: 3, Name: "Lantern Oath", Text: "You carry a lantern that never gutters. You can cast Dancing Lights with it."},
		{Level: 3, Name: "Kindle", Text: "As a Bonus Action you brighten the lantern. You can cast Light with it, spending a use of Lantern Light."},
		{Level: 7, Name: "Steady Flame", Text: "Your lantern burns through magical darkness. It spends a use of Embers."},
	}
	if !reflect.DeepEqual(sc.Traits, want) {
		t.Fatalf("traits = %+v", sc.Traits)
	}
	if len(sc.Resources) != 4 {
		t.Fatalf("resources = %+v", sc.Resources)
	}
	light, embers, glow, will := sc.Resources[0], sc.Resources[1], sc.Resources[2], sc.Resources[3]
	if light.Slug != "hb-0190c7a80000-light" || light.Owner != owner || light.Name != "Lantern Light" {
		t.Fatalf("light = %+v", light)
	}
	stats := features.Stats{Level: 7, AbilityMod: 3, Proficiency: 3}
	for _, c := range []struct {
		r    features.Resource
		most int
	}{{light, 3}, {embers, 2}, {glow, 14}, {will, 3}} {
		if got := c.r.Max(stats); got != c.most {
			t.Errorf("%s max = %d, want %d", c.r.Name, got, c.most)
		}
	}
	if embers.Max(features.Stats{Level: 6, AbilityMod: 0, Proficiency: 3}) != 0 || will.Ability != "wisdom" {
		t.Fatal("Embers come at level 7, Will reads Wisdom")
	}
	if die, ok := embers.DieAt(7); !ok || die != "d6" {
		t.Fatalf("embers die = %q", die)
	}
	if _, ok := light.DieAt(7); ok {
		t.Fatal("Lantern Light rolls no die")
	}
	if embers.Regain(features.ShortRest, stats, 0, 0) != 2 || embers.Regain(features.LongRest, stats, 0, 0) != 2 {
		t.Fatal("a short-rest Resource comes back on either rest")
	}
	if light.Regain(features.ShortRest, stats, 0, 0) != 0 || light.Regain(features.LongRest, stats, 0, 0) != 3 || glow.Regain(features.Dawn, stats, 0, 1) != 14 {
		t.Fatal("a long-rest Resource waits for a long rest; a dawn one for dawn")
	}
	if !reflect.DeepEqual(sc.Choices, []features.Choice{{
		Slug: "hb-0190c7a80000-lantern-style", Name: "Lantern Style", Level: 3, Count: 1, Pool: features.Listed, From: "",
		Options: []features.Option{{Slug: "bog-glass", Name: "Bog Glass"}, {Slug: "ember-wick", Name: "Ember Wick"}},
	}}) {
		t.Fatalf("choices = %+v", sc.Choices)
	}
	lines := strings.Join(subclassbuild.Lines(sc), "\n")
	for _, w := range []string{"Fighter subclass", "Level 3: Lantern Oath. You carry", "Level 3 choice: Lantern Style, pick 1 of Bog Glass, Ember Wick.", "Lantern Light: uses equal to your Proficiency Bonus from level 3, back on a Long Rest.", "Embers: 2 uses from level 7, each rolls a d6, back on a Short or Long Rest.", "Glow: 2 uses per fighter level from level 3, back at dawn.", "Will: uses equal to your Wisdom modifier (at least 1) from level 3, back on a Long Rest."} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
}

func TestMergingAddsHomebrewToTheCatalogWithoutTouchingIt(t *testing.T) {
	t.Parallel()
	base := features.Catalog{
		Resources:     map[string]features.Resource{"rage": {Slug: "rage"}},
		Scales:        map[string]features.Named{},
		Choices:       map[features.Owner][]features.Choice{{Kind: "class", Slug: "fighter"}: {{Slug: "subclass"}}},
		Prerequisites: map[features.Owner][]features.Requirement{},
	}
	sc := subclassbuild.Compile("hb-0190c7a80000", "Lantern Warden", warden())
	cat := subclassbuild.Merge(base, []subclassbuild.Subclass{sc})
	if len(base.Resources) != 1 || len(base.Choices) != 1 {
		t.Fatal("the compendium's catalog is left as it was")
	}
	if _, ok := cat.Resources["hb-0190c7a80000-embers"]; !ok || len(cat.Resources) != 5 {
		t.Fatalf("resources = %v", cat.Resources)
	}
	if got := cat.Choices[features.Owner{Kind: "subclass", Slug: "hb-0190c7a80000"}]; len(got) != 1 || len(cat.Choices) != 2 {
		t.Fatalf("choices = %v", cat.Choices)
	}
}

func TestDesignsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 1001)
	for want, change := range map[string]func(*subclassbuild.Design){
		"choose the class":             func(d *subclassbuild.Design) { d.Class = "bard" },
		"give it 1 to 40 features":     func(d *subclassbuild.Design) { d.Features = nil },
		"from level 1 to 20":           func(d *subclassbuild.Design) { d.Features[0].Level = 21 },
		"name each feature":            func(d *subclassbuild.Design) { d.Features[0].Name = " " },
		"in up to 1000 characters":     func(d *subclassbuild.Design) { d.Features[0].Text = long },
		"uses no Resource called nope": func(d *subclassbuild.Design) { d.Features[0].Uses = "nope" },
		"by its slug":                  func(d *subclassbuild.Design) { d.Features[1].Spell = "Light!" },
		"name the spell":               func(d *subclassbuild.Design) { d.Features[1].SpellName = "" },
		"up to 10 Resources":           func(d *subclassbuild.Design) { d.Resources = append(d.Resources, make([]subclassbuild.Resource, 7)...) },
		"key each Resource":            func(d *subclassbuild.Design) { d.Resources[0].Key = "Light" },
		"once each":                    func(d *subclassbuild.Design) { d.Resources[1].Key = "light" },
		"name each Resource":           func(d *subclassbuild.Design) { d.Resources[0].Name = "" },
		"counts its uses":              func(d *subclassbuild.Design) { d.Resources[0].Basis = "moon" },
		"1 to 20 uses":                 func(d *subclassbuild.Design) { d.Resources[1].Amount = 0 },
		"1 to 10 uses per level":       func(d *subclassbuild.Design) { d.Resources[2].Amount = 11 },
		"choose the ability":           func(d *subclassbuild.Design) { d.Resources[3].Ability = "luck" },
		"comes from level 1 to 20":     func(d *subclassbuild.Design) { d.Resources[0].FromLevel = 0 },
		"a die from d4 to d12":         func(d *subclassbuild.Design) { d.Resources[1].Die = "d7" },
		"comes back on":                func(d *subclassbuild.Design) { d.Resources[0].Recharge = "never" },
		"up to 10 choices":             func(d *subclassbuild.Design) { d.Choices = append(d.Choices, make([]subclassbuild.Choice, 10)...) },
		"a choice at level 1 to 20":    func(d *subclassbuild.Design) { d.Choices[0].Level = 0 },
		"name each choice":             func(d *subclassbuild.Design) { d.Choices[0].Name = "!!" },
		"two choices by one name":      func(d *subclassbuild.Design) { d.Choices = append(d.Choices, d.Choices[0]) },
		"offer 2 to 20 options":        func(d *subclassbuild.Design) { d.Choices[0].Options = []string{"Bog Glass"} },
		"name each option once":        func(d *subclassbuild.Design) { d.Choices[0].Options = []string{"Bog Glass", "bog glass"} },
		"pick 1 to 2 options":          func(d *subclassbuild.Design) { d.Choices[0].Count = 3 },
	} {
		d := warden()
		change(&d)
		err := subclassbuild.Check(d, classes)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	d := warden()
	d.Features = make([]subclassbuild.Feature, 41)
	if err := subclassbuild.Check(d, classes); err == nil || !strings.Contains(err.Error(), "1 to 40 features") {
		t.Errorf("41 features: %v", err)
	}
	d = warden()
	d.Choices[0].Options = []string{"A", "B", strings.Repeat("o", 61)}
	if err := subclassbuild.Check(d, classes); err == nil || !strings.Contains(err.Error(), "up to 60 characters") {
		t.Errorf("a long option: %v", err)
	}
}

func TestDesignsAtTheLimits(t *testing.T) {
	t.Parallel()
	d := warden()
	d.Features[0].Name = strings.Repeat("n", 60)
	d.Choices[0].Count = 2
	if err := subclassbuild.Check(d, classes); err != nil {
		t.Fatal(err)
	}
}
