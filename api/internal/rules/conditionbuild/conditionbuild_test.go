package conditionbuild_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func part(kind string, values effects.Values) effects.Authored {
	return effects.Authored{Kind: kind, Values: values, Groups: nil}
}

func edge(against, advantage bool) effects.Authored {
	return part("edge", effects.Values{"against": {Flag: against}, "advantage": {Flag: advantage}, "range": {Text: "any"}})
}

func saveEdge(ability, mode string) effects.Authored {
	return part("save_edge", effects.Values{"ability": {Text: ability}, "mode": {Text: mode}})
}

func frostbite() conditionbuild.Design {
	return conditionbuild.Design{
		Icon: "snow", Color: "#7fa8dd", Text: "Cold seeps into the bones.", Ends: "rest", Ability: "",
		Stacks: true, MaxLevel: 4, PerLevel: conditionbuild.Penalty{D20: 1, SpeedFt: 5, DeathAt: 0},
		Parts: []effects.Authored{
			edge(false, false),
			edge(true, true),
			saveEdge("dexterity", "disadvantage"),
			saveEdge("strength", "fail"),
			saveEdge("constitution", "advantage"),
			edge(false, true),
			edge(true, false),
			part("incapacitated", nil),
			part("immobile", nil),
			part("speed_penalty", effects.Values{"ft": {Number: 10}}),
			part("crit_within", effects.Values{"feet": {Number: 5}}),
			part("manual", effects.Values{"instruction": {Text: "The DM decides what the cold does to gear."}}),
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
	want := []effects.Part{
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
	if !reflect.DeepEqual(def.Parts, want) {
		t.Fatalf("parts = %+v", def.Parts)
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
	if len(p.Definition.Parts) != 0 || p.Definition.Duration.RepeatSave != "wisdom" || p.Definition.Duration.Kind != effects.UntilDispelled {
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
		Parts: []effects.Authored{part("speed_penalty", effects.Values{"ft": {Number: 10}})},
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
		t.Run(want, func(t *testing.T) {
			bad := d
			change(&bad)
			if err := conditionbuild.Check(bad); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %v", want, err)
			}
		})
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
		"up to 20 Parts":              func(d *conditionbuild.Design) { d.Parts = make([]effects.Authored, 21) },
		"a condition holds Parts of the kinds crit_within, edge, immobile, incapacitated, manual, save_edge, speed_penalty": func(d *conditionbuild.Design) { d.Parts[0].Kind = "summon" },
		"part 3: ability: choose one of the options":    func(d *conditionbuild.Design) { d.Parts[2].Values = effects.Values{"mode": {Text: "fail"}} },
		"part 10: ft: runs from 0 to 120":               func(d *conditionbuild.Design) { d.Parts[9].Values = effects.Values{"ft": {Number: 125}} },
		"part 12: instruction: write up to 500 letters": func(d *conditionbuild.Design) { d.Parts[11].Values = nil },
		"a condition does not scale": func(d *conditionbuild.Design) {
			d.Parts[9].Scaling = []effects.AuthoredScaling{{Field: "ft", Axis: "slot_level", Rule: "linear", Base: 1, Per: &effects.Value{Number: 5}}}
		},
		"part 9: groups: Speed 0 holds no group named then": func(d *conditionbuild.Design) {
			d.Parts[8].Groups = []effects.AuthoredGroup{{Name: "then", Parts: nil}}
		},
	} {
		t.Run(want, func(t *testing.T) {
			d := frostbite()
			change(&d)
			if err := conditionbuild.Check(d); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %v", want, err)
			}
		})
	}
	d := frostbite()
	d.Text, d.MaxLevel, d.PerLevel = strings.Repeat("x", 2000), 10, conditionbuild.Penalty{D20: 5, SpeedFt: 30, DeathAt: 10}
	d.Parts[9].Values, d.Parts[10].Values = effects.Values{"ft": {Number: 120}}, effects.Values{"feet": {Number: 0}}
	for len(d.Parts) < 20 {
		d.Parts = append(d.Parts, part("incapacitated", nil))
	}
	d.Parts[11].Values = effects.Values{"instruction": {Text: strings.Repeat("m", 500)}}
	if err := conditionbuild.Check(d); err != nil {
		t.Fatalf("a design at its limits: %v", err)
	}
	d.MaxLevel, d.PerLevel.DeathAt = 2, 2
	if err := conditionbuild.Check(d); err != nil {
		t.Fatalf("a design at its low limits: %v", err)
	}
	d.Parts[11].Values = effects.Values{"instruction": {Text: strings.Repeat("m", 501)}}
	if err := conditionbuild.Check(d); err == nil {
		t.Fatal("a manual part of 501 characters builds")
	}
}

// A condition serves the kinds it holds, from the list it checks Parts against; none holds Parts beneath.
func TestAConditionServesWhatItHolds(t *testing.T) {
	t.Parallel()
	want := effects.Holds{
		Kinds:   []string{"crit_within", "edge", "immobile", "incapacitated", "manual", "save_edge", "speed_penalty"},
		Beneath: []string{}, Fills: []string{}, Narrows: []effects.Narrow{},
	}
	if got := conditionbuild.Holds(); !reflect.DeepEqual(got, want) {
		t.Errorf("holds %+v, want %+v", got, want)
	}
}
