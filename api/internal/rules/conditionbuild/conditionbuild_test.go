package conditionbuild_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func frostbite() conditionbuild.Design {
	return conditionbuild.Design{
		Icon: "snow", Color: "#7fa8dd", Text: "Cold seeps into the bones.", Ends: "rest", Ability: "",
		Stacks: true, MaxLevel: 4, PerLevel: conditionbuild.Penalty{D20: 1, SpeedFt: 5, DeathAt: 0},
		Parts: []conditionbuild.Part{
			{Type: "attack_disadvantage"},
			{Type: "attacked_advantage"},
			{Type: "save_disadvantage", Ability: "dexterity"},
			{Type: "save_fails", Ability: "strength"},
			{Type: "save_advantage", Ability: "constitution"},
			{Type: "attack_advantage"},
			{Type: "attacked_disadvantage"},
			{Type: "incapacitated"},
			{Type: "immobile"},
			{Type: "speed_penalty", Feet: 10},
			{Type: "crit_within", Feet: 5},
			{Type: "manual", Text: "The DM decides what the cold does to gear."},
		},
	}
}

func TestFrostbiteBuildsIntoAStackingCondition(t *testing.T) {
	t.Parallel()
	d := frostbite()
	if err := conditionbuild.Check(d); err != nil {
		t.Fatal(err)
	}
	c := conditionbuild.Compile("hb-0190c7a80005", "Frostbite", d)
	def := c.Definition
	if def.Slug != "hb-0190c7a80005" || def.Name != "Frostbite" || def.Owner != effects.OwnedByCondition || def.Duration != (effects.Duration{Kind: effects.UntilRest, Amount: 0, RepeatSave: ""}) {
		t.Fatalf("definition = %+v", def)
	}
	want := []effects.Component{
		effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 0, MaxLevel: 4},
		effects.Edge{Against: false, Advantage: false, Range: effects.AnyRange, SourceOnly: false},
		effects.Edge{Against: true, Advantage: true, Range: effects.AnyRange, SourceOnly: false},
		effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveDisadvantage},
		effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
		effects.SaveEdge{Ability: "constitution", Mode: effects.SaveAdvantage},
		effects.Edge{Against: false, Advantage: true, Range: effects.AnyRange, SourceOnly: false},
		effects.Edge{Against: true, Advantage: false, Range: effects.AnyRange, SourceOnly: false},
		effects.Incapacitated{},
		effects.Immobile{},
		effects.SpeedPenalty{Ft: 10},
		effects.CritWithin{Feet: 5},
		effects.Manual{Instruction: "The DM decides what the cold does to gear."},
	}
	if !reflect.DeepEqual(def.Components, want) {
		t.Fatalf("components = %+v", def.Components)
	}
	if c.Icon != "snow" || c.Color != "#7fa8dd" || c.Text != "Cold seeps into the bones." {
		t.Fatalf("look = %+v", c)
	}
	cat := effects.Catalog{def.Slug: def}
	if !cat.Stacks(def.Slug) || cat.MaxLevel(def.Slug) != 4 {
		t.Fatal("frostbite stacks to 4")
	}
	two := []effects.Active{{Slug: def.Slug, Source: "", Level: 2, Mode: ""}}
	if cat.SpeedPenaltyFt(two) != 20 || !cat.Incapacitated(two) {
		t.Fatalf("two levels: speed -%d", cat.SpeedPenaltyFt(two))
	}
	lines := strings.Join(conditionbuild.Lines(c), "\n")
	for _, w := range []string{"Cold seeps into the bones.", "Ends on a rest.", "Each level gives a −1 penalty to D20 Tests and reduces Speed by 5 feet; it rises to level 4 at most."} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
	plain := frostbite()
	plain.Stacks, plain.Parts, plain.Text, plain.Ends, plain.Ability = false, nil, "", "save", "wisdom"
	p := conditionbuild.Compile("hb-p", "Dazed", plain)
	if len(p.Definition.Components) != 0 || p.Definition.Duration.RepeatSave != "wisdom" || p.Definition.Duration.Kind != effects.UntilDispelled {
		t.Fatalf("a plain condition = %+v", p.Definition)
	}
	if got := conditionbuild.Lines(p); !reflect.DeepEqual(got, []string{"Ends when the creature succeeds on a Wisdom saving throw at the end of its turn."}) {
		t.Fatalf("plain lines = %v", got)
	}
	plain.Ends = "removed"
	if got := conditionbuild.Lines(conditionbuild.Compile("x", "X", plain)); !reflect.DeepEqual(got, []string{"Lasts until removed."}) {
		t.Fatalf("removed lines = %v", got)
	}
}

// A lingering injury is a condition that lasts until its cure: it names the cure, no rest ends it, and
// it reads back as lingering.
func TestALingeringInjuryLastsUntilItsCure(t *testing.T) {
	t.Parallel()
	d := conditionbuild.Design{
		Icon: "skull", Color: "#aa3344", Text: "The leg never set right.", Ends: "cure", Cure: "  Regenerate, or a month of rest ", PerLevel: conditionbuild.Penalty{},
		Parts: []conditionbuild.Part{{Type: "speed_penalty", Feet: 10}},
	}
	if err := conditionbuild.Check(d); err != nil {
		t.Fatal(err)
	}
	c := conditionbuild.Compile("hb-limp", "Limp", d)
	if c.Definition.Duration != (effects.Duration{Kind: effects.UntilCured, Amount: 0, RepeatSave: ""}) || !c.Definition.Duration.Lingers() || c.Definition.Duration.EndsOnRest() {
		t.Fatalf("duration = %+v", c.Definition.Duration)
	}
	if c.Cure != "Regenerate, or a month of rest" {
		t.Fatalf("cure = %q", c.Cure)
	}
	lines := conditionbuild.Lines(c)
	if !slices.Contains(lines, "Lingers through every rest until cured: Regenerate, or a month of rest.") || slices.Contains(lines, "Lasts until removed.") {
		t.Fatalf("lines = %q", lines)
	}
	// Any other condition has no cure to name, and says nothing of one.
	plain := conditionbuild.Compile("hb-x", "X", conditionbuild.Design{Icon: "skull", Color: "#aa3344", Ends: "removed", Cure: "", Parts: nil})
	if plain.Cure != "" || plain.Definition.Duration.Lingers() || !slices.Contains(conditionbuild.Lines(plain), "Lasts until removed.") {
		t.Fatalf("a plain condition = %+v", plain)
	}
	for want, change := range map[string]func(*conditionbuild.Design){
		"name its cure":               func(d *conditionbuild.Design) { d.Cure = "  " },
		"in up to 80 characters":      func(d *conditionbuild.Design) { d.Cure = strings.Repeat("x", 81) },
		"only a condition that lasts": func(d *conditionbuild.Design) { d.Ends = "rest" },
	} {
		bad := d
		change(&bad)
		if err := conditionbuild.Check(bad); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	longest := d
	longest.Cure = strings.Repeat("x", 80)
	if err := conditionbuild.Check(longest); err != nil {
		t.Errorf("the longest cure: %v", err)
	}
}

func TestExhaustionVariants(t *testing.T) {
	t.Parallel()
	for variant, want := range map[string]effects.Exhausting{
		"srd-2024": {D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 6},
		"gentle":   {D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 10, MaxLevel: 10},
		"grim":     {D20PerLevel: 2, SpeedFtPerLevel: 10, DeathAt: 4, MaxLevel: 4},
		"off":      {D20PerLevel: 0, SpeedFtPerLevel: 0, DeathAt: 0, MaxLevel: 1},
	} {
		got, ok := conditionbuild.Exhaustion(variant)
		if !ok || got != want {
			t.Errorf("%s = %+v %v", variant, got, ok)
		}
	}
	if _, ok := conditionbuild.Exhaustion("brutal"); ok {
		t.Fatal("an unknown variant")
	}
	if v := conditionbuild.Variants(); len(v) != 4 || v[0] != "srd-2024" {
		t.Fatalf("variants = %v", v)
	}
}

func TestConditionDesignsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(*conditionbuild.Design){
		"choose an icon":              func(d *conditionbuild.Design) { d.Icon = "rocket" },
		"a colour like #7fa8dd":       func(d *conditionbuild.Design) { d.Color = "blue" },
		"in up to 2000 characters":    func(d *conditionbuild.Design) { d.Text = strings.Repeat("x", 2001) },
		"on a rest, until a save, or": func(d *conditionbuild.Design) { d.Ends = "never" },
		"the ability its save uses":   func(d *conditionbuild.Design) { d.Ends, d.Ability = "save", "luck" },
		"stack 2 to 10 levels":        func(d *conditionbuild.Design) { d.MaxLevel = 1 },
		"a −0 to −5 penalty":          func(d *conditionbuild.Design) { d.PerLevel.D20 = 6 },
		"0 to 30 feet per level":      func(d *conditionbuild.Design) { d.PerLevel.SpeedFt = 31 },
		"kill at a level up to its":   func(d *conditionbuild.Design) { d.PerLevel.DeathAt = 5 },
		"up to 20 parts":              func(d *conditionbuild.Design) { d.Parts = make([]conditionbuild.Part, 21) },
		"choose each part":            func(d *conditionbuild.Design) { d.Parts[0].Type = "explode" },
		"the ability each save part":  func(d *conditionbuild.Design) { d.Parts[2].Ability = "" },
		"5 to 60 feet":                func(d *conditionbuild.Design) { d.Parts[9].Feet = 0 },
		"describe each manual part":   func(d *conditionbuild.Design) { d.Parts[11].Text = "" },
	} {
		d := frostbite()
		change(&d)
		if err := conditionbuild.Check(d); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	d := frostbite()
	d.Text, d.MaxLevel, d.PerLevel, d.Parts[9].Feet, d.Parts[10].Feet = strings.Repeat("x", 2000), 10, conditionbuild.Penalty{D20: 5, SpeedFt: 30, DeathAt: 10}, 60, 5
	d.Parts = append(d.Parts, make([]conditionbuild.Part, 8)...)
	for i := 12; i < 20; i++ {
		d.Parts[i] = conditionbuild.Part{Type: "incapacitated"}
	}
	d.Parts[11].Text = strings.Repeat("m", 500)
	if err := conditionbuild.Check(d); err != nil {
		t.Fatalf("a design at its limits: %v", err)
	}
	d.MaxLevel, d.PerLevel.DeathAt = 2, 2
	if err := conditionbuild.Check(d); err != nil {
		t.Fatalf("a design at its low limits: %v", err)
	}
	d.Parts[11].Text = strings.Repeat("m", 501)
	if err := conditionbuild.Check(d); err == nil {
		t.Fatal("a manual part of 501 characters builds")
	}
}
