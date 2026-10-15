package effects_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// grown is the dice of an Effect's one save_damage Part once it is activated at a level.
func grown(s effects.Scaling, dice string, l effects.Level) string {
	s.Part, s.Field = 0, "dice"
	d := effects.Definition{Parts: []effects.Part{effects.SaveDamage{Ability: "dexterity", Dice: dice, Type: "fire", Half: true}}, Scaling: []effects.Scaling{s}}
	return d.At(l).Parts[0].(effects.SaveDamage).Dice
}

// Each rule grows a value its own way, by the level of its axis; no level leaves the value as written.
func TestScalingGrowsAValueByItsRule(t *testing.T) {
	t.Parallel()
	fireball := effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Linear{Base: 3, Per: effects.Value{Text: "1d6"}}}
	for slot, want := range map[int]string{0: "8d6", 2: "8d6", 3: "8d6", 4: "9d6", 5: "10d6"} {
		if got := grown(fireball, "8d6", effects.Level{Slot: slot}); got != want {
			t.Errorf("fireball at slot %d = %s", slot, got)
		}
	}
	if got := grown(effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "1d8"}}}, "2d6", effects.Level{Slot: 2}); got != "2d6+1d8" {
		t.Errorf("unlike dice = %s", got)
	}
	for _, bad := range []string{"x", "2d6", "ad6", "2d6+1"} {
		if got := grown(effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: bad}}}, "bd6", effects.Level{Slot: 2}); got != "bd6+"+bad {
			t.Errorf("odd dice %q = %s", bad, got)
		}
	}
	cantrip := effects.Scaling{Axis: effects.CharacterLevel, Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "2d10"}}, {At: 11, Value: effects.Value{Text: "3d10"}}, {At: 17, Value: effects.Value{Text: "4d10"}}}}}
	for level, want := range map[int]string{0: "1d10", 4: "1d10", 5: "2d10", 10: "2d10", 11: "3d10", 17: "4d10", 20: "4d10"} {
		if got := grown(cantrip, "1d10", effects.Level{Character: level, Slot: 9}); got != want {
			t.Errorf("cantrip at %d = %s", level, got)
		}
	}
	aura := effects.Scaling{Axis: effects.ClassLevel, Class: "paladin", Rule: effects.Table{Steps: []effects.Step{{At: 18, Value: effects.Value{Text: "2d8"}}}}}
	if grown(aura, "1d8", effects.Level{Character: 20, Classes: map[string]int{"paladin": 17}}) != "1d8" || grown(aura, "1d8", effects.Level{Classes: map[string]int{"paladin": 18}}) != "2d8" {
		t.Error("class levels count only that class")
	}
	sneak := effects.Scaling{Axis: effects.TableColumn, Class: "rogue", Rule: effects.Column{Name: "Sneak Attack"}}
	if grown(sneak, "1d6", effects.Level{Columns: map[string]string{"Sneak Attack": "3d6"}}) != "3d6" || grown(sneak, "1d6", effects.Level{Slot: 9}) != "1d6" {
		t.Error("a table column replaces the value when it has one")
	}
	surge := effects.Scaling{Axis: effects.Charges, Rule: effects.Linear{Base: 1, Per: effects.Value{Text: "1d6"}}}
	if grown(surge, "2d6", effects.Level{Charges: 3, Slot: 9}) != "4d6" || grown(surge, "2d6", effects.Level{Slot: 9}) != "2d6" {
		t.Error("charges spent past the first add dice")
	}
	if grown(effects.Scaling{Axis: "phase_of_moon", Rule: effects.Linear{Base: 0, Per: effects.Value{Text: "1d6"}}}, "1d4", effects.Level{Slot: 9, Character: 9, Charges: 3}) != "1d4" {
		t.Error("an axis the rules do not know has no level")
	}
}

// numbered is an Effect whose one temp_hp Part grows by s, activated at l.
func numbered(s effects.Scaling, amount int, l effects.Level) int {
	s.Part, s.Field = 0, "amount"
	d := effects.Definition{Parts: []effects.Part{effects.TempHP{Amount: amount}}, Scaling: []effects.Scaling{s}}
	return d.At(l).Parts[0].(effects.TempHP).Amount
}

// Numbers grow by a rule too, and may follow a formula of the level.
func TestNumbersGrowAndFollowFormulas(t *testing.T) {
	t.Parallel()
	if got := numbered(effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: effects.Value{Number: 5}}}, 9, effects.Level{Slot: 3}); got != 19 {
		t.Errorf("false life at slot 3 = %d", got)
	}
	steps := effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Number: 20}}}}}
	if numbered(steps, 10, effects.Level{Slot: 4}) != 10 || numbered(steps, 10, effects.Level{Slot: 5}) != 20 {
		t.Error("a table of numbers")
	}
	beast := effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Formula{Expr: "30 + 10 * (level - 3)"}}
	for slot, want := range map[int]int{0: 30, 3: 30, 4: 40, 9: 90} {
		if got := numbered(beast, 30, effects.Level{Slot: slot}); got != want {
			t.Errorf("a formula at slot %d = %d", slot, got)
		}
	}
	if numbered(effects.Scaling{Axis: effects.SlotLevel, Rule: effects.Formula{Expr: "level +"}}, 7, effects.Level{Slot: 4}) != 7 {
		t.Error("a formula that does not read leaves the value")
	}
	rage := effects.Scaling{Axis: effects.TableColumn, Class: "barbarian", Rule: effects.Column{Name: "Rage Damage"}}
	if numbered(rage, 2, effects.Level{Columns: map[string]string{"Rage Damage": "3"}}) != 3 || numbered(rage, 2, effects.Level{Columns: map[string]string{"Rage Damage": "1d6"}}) != 2 {
		t.Error("a numeric column sets a number; any other leaves it")
	}
}

// Formulas are whole numbers, the level, + - * and brackets, and nothing else.
func TestFormulasReadOnlyArithmeticOfTheLevel(t *testing.T) {
	t.Parallel()
	for expr, want := range map[string]int{
		"11 + level": 15, "level": 4, "30 + 10 * (level - 3)": 40, "2*level*3": 24, "(level)": 4, " 7 ": 7, "20 - level - 1": 15,
		"((level + 1) * 2)": 10, "9999": 9999, "level-1+2": 5,
	} {
		if got, ok := effects.Evaluate(expr, 4); !ok || got != want {
			t.Errorf("%q = %d %v", expr, got, ok)
		}
	}
	for _, bad := range []string{"", "level +", "+ 1", "(level", "level)", "2 ** 3", "level / 2", "lvl", "levels", "12345", "1 2", "level(2)", "()", "-1", "2 + l", "x"} {
		if got, ok := effects.Evaluate(bad, 4); ok {
			t.Errorf("%q reads as %d", bad, got)
		}
	}
}

// Activating an Effect applies every Scaling, to the Part it names counted depth first and to the
// Activation's own values, and leaves the Effect as written untouched.
func TestAnEffectIsActivatedAtALevel(t *testing.T) {
	t.Parallel()
	slot := func(part int, field string, r effects.Rule) effects.Scaling {
		return effects.Scaling{Part: part, Field: field, Axis: effects.SlotLevel, Class: "", Rule: r}
	}
	plus := func(base int, v effects.Value) effects.Rule { return effects.Linear{Base: base, Per: v} }
	d := effects.Definition{
		Slug: "storm", Name: "Storm", Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1},
		Parts: []effects.Part{
			effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 60},
			effects.SaveDamage{Ability: "dexterity", Dice: "3d6", Type: "fire", Half: true},
			effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 20}, Then: []effects.Part{effects.TempHP{Amount: 1}, effects.ExtraDamage{Dice: "1d4"}}},
			effects.Choice{Modes: []effects.Group{
				{Name: "Wolves", Parts: []effects.Part{effects.Summon{Monster: "wolf", Count: 1}}},
				{Name: "Calm", Parts: []effects.Part{effects.Manual{Instruction: "All is calm."}}},
			}},
		},
		Scaling: []effects.Scaling{
			slot(0, "size_ft", plus(3, effects.Value{Number: 5})),
			slot(0, "range_ft", plus(3, effects.Value{Number: 30})),
			slot(1, "dice", plus(3, effects.Value{Text: "1d6"})),
			slot(2, "n", plus(3, effects.Value{Number: 10})),
			slot(3, "amount", effects.Formula{Expr: "level * 2"}),
			slot(4, "dice", effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "2d4"}}}}),
			slot(6, "count", plus(3, effects.Value{Number: 2})),
			slot(effects.OnActivation, "concentration", effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Flag: false}}}}),
			slot(effects.OnActivation, "duration", effects.Table{Steps: []effects.Step{{At: 4, Value: effects.Value{Number: 8}}}}),
			slot(effects.OnActivation, "duration_unit", effects.Table{Steps: []effects.Step{{At: 4, Value: effects.Value{Text: "hours"}}}}),
			slot(effects.OnActivation, "range", plus(1, effects.Value{Number: 99})),
			slot(9, "dice", plus(1, effects.Value{Text: "1d6"})),
			slot(7, "instruction", plus(1, effects.Value{Text: "1d6"})),
		},
	}
	written := d.Text()
	want := d
	want.Concentration, want.Duration = false, effects.Duration{Kind: effects.Hours, Amount: 8}
	want.Parts = []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 30, RangeFt: 120},
		effects.SaveDamage{Ability: "dexterity", Dice: "5d6", Type: "fire", Half: true},
		effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 40}, Then: []effects.Part{effects.TempHP{Amount: 10}, effects.ExtraDamage{Dice: "2d4"}}},
		effects.Choice{Modes: []effects.Group{
			{Name: "Wolves", Parts: []effects.Part{effects.Summon{Monster: "wolf", Count: 5}}},
			{Name: "Calm", Parts: []effects.Part{effects.Manual{Instruction: "All is calm."}}},
		}},
	}
	if got := d.At(effects.Level{Slot: 5}); !reflect.DeepEqual(got, want) {
		t.Fatalf("at slot 5:\n got %+v\nwant %+v", got, want)
	}
	if got := d.At(effects.Level{}); !reflect.DeepEqual(got, d) {
		t.Fatalf("at no level:\n got %+v\nwant %+v", got, d)
	}
	if !reflect.DeepEqual(d.Text(), written) || d.Parts[1].(effects.SaveDamage).Dice != "3d6" {
		t.Fatal("activating an Effect changes the Effect as written")
	}
	if got := (effects.Definition{}).At(effects.Level{Slot: 9}); got.Parts != nil {
		t.Fatalf("an Effect of no Parts = %+v", got)
	}
}

// A kind marks the fields that scale, each one it describes and of a sort that can grow; a formula
// only ever fills a number. Its Parts grow those fields and no other.
func TestKindsMarkTheFieldsThatScale(t *testing.T) {
	t.Parallel()
	want := map[string][]effects.Scalable{
		"area":         {{Field: "size_ft", Formula: false}, {Field: "range_ft", Formula: false}},
		"branch":       {{Field: "n", Formula: false}},
		"extra_damage": {{Field: "dice", Formula: false}},
		"save_damage":  {{Field: "dice", Formula: false}},
		"summon":       {{Field: "count", Formula: false}},
		"temp_hp":      {{Field: "amount", Formula: true}},
	}
	for _, k := range effects.Kinds() {
		d := k.Describe()
		got := effects.ScalesOf(k)
		if !reflect.DeepEqual(got, want[d.Slug]) {
			t.Errorf("%s scales %+v", d.Slug, got)
		}
		for _, s := range got {
			i := slices.IndexFunc(d.Fields, func(f effects.Field) bool { return f.Name == s.Field })
			if i < 0 || !slices.Contains([]effects.FieldType{effects.DiceField, effects.NumberField}, d.Fields[i].Type) || (s.Formula && d.Fields[i].Type != effects.NumberField) {
				t.Errorf("%s scales a field it cannot grow: %+v", d.Slug, s)
			}
		}
	}
	grow := func(v effects.Value) effects.Value { return effects.Value{Text: v.Text + "+1d4", Number: v.Number + 1} }
	for _, c := range []struct {
		p      effects.Part
		fields map[string]effects.Part
	}{
		{effects.Area{Shape: hex.ConeArea, SizeFt: 15, RangeFt: 5}, map[string]effects.Part{
			"size_ft": effects.Area{Shape: hex.ConeArea, SizeFt: 16, RangeFt: 5}, "range_ft": effects.Area{Shape: hex.ConeArea, SizeFt: 15, RangeFt: 6},
		}},
		{effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}}, map[string]effects.Part{"n": effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 6}}}},
		{effects.ExtraDamage{Dice: "1d6"}, map[string]effects.Part{"dice": effects.ExtraDamage{Dice: "1d6+1d4"}}},
		{effects.SaveDamage{Ability: "wisdom", Dice: "2d8", Type: "cold"}, map[string]effects.Part{"dice": effects.SaveDamage{Ability: "wisdom", Dice: "2d8+1d4", Type: "cold"}}},
		{effects.Summon{Monster: "wolf", Count: 2, Shares: true}, map[string]effects.Part{"count": effects.Summon{Monster: "wolf", Count: 3, Shares: true}}},
		{effects.TempHP{Amount: 4}, map[string]effects.Part{"amount": effects.TempHP{Amount: 5}}},
	} {
		p := c.p
		g := p.(effects.Grower)
		for field, want := range c.fields {
			if got := g.Grow(field, grow); !reflect.DeepEqual(got, want) {
				t.Errorf("%s grows %s to %+v", p.Kind(), field, got)
			}
		}
		if got := g.Grow("colour", grow); !reflect.DeepEqual(got, p) {
			t.Errorf("%s grows a field it does not have: %+v", p.Kind(), got)
		}
	}
}

// The Activation's own values scale too: how long the Effect lasts, in what unit, and whether it needs
// concentration.
func TestTheActivationScales(t *testing.T) {
	t.Parallel()
	fields := effects.ActivationFields()
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	if !reflect.DeepEqual(names, []string{"duration", "duration_unit", "concentration"}) {
		t.Fatalf("activation fields = %+v", fields)
	}
	if unit := fields[1]; unit.Type != effects.ChoiceField || !slices.Contains(unit.Options, "until_dispelled") || fields[0].Type != effects.NumberField || fields[2].Type != effects.FlagField {
		t.Fatalf("activation fields = %+v", fields)
	}
	want := []effects.Scalable{{Field: "duration", Formula: true}, {Field: "duration_unit", Formula: false}, {Field: "concentration", Formula: false}}
	if got := effects.ActivationScales(); !reflect.DeepEqual(got, want) {
		t.Fatalf("activation scales = %+v", got)
	}
	if got := effects.Axes(); !reflect.DeepEqual(got, []string{"slot_level", "character_level", "class_level", "table_column", "charges"}) {
		t.Fatalf("axes = %v", got)
	}
}

// Each Scaling reads as a rules line naming what grows, by how much, and with what.
func TestScalingReadsAsRulesText(t *testing.T) {
	t.Parallel()
	steps := func(vs ...effects.Value) effects.Rule {
		out := effects.Table{}
		for i, v := range vs {
			out.Steps = append(out.Steps, effects.Step{At: 5 + 6*i, Value: v})
		}
		return out
	}
	dice := func(s string) effects.Value { return effects.Value{Text: s} }
	n := func(i int) effects.Value { return effects.Value{Number: i} }
	on := func(a effects.Axis, class string, r effects.Rule) effects.Scaling {
		return effects.Scaling{Part: 1, Field: "dice", Axis: a, Class: class, Rule: r}
	}
	act := func(field string, r effects.Rule) effects.Scaling {
		return effects.Scaling{Part: effects.OnActivation, Field: field, Axis: effects.SlotLevel, Class: "", Rule: r}
	}
	for want, s := range map[string]effects.Scaling{
		"Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3.":                on(effects.SlotLevel, "", effects.Linear{Base: 3, Per: dice("1d6")}),
		"Cantrip Upgrade: the damage becomes 2d10 at level 5, 3d10 at level 11, 4d10 at level 17.":                 on(effects.CharacterLevel, "", steps(dice("2d10"), dice("3d10"), dice("4d10"))),
		"The damage becomes 2d8 at Paladin level 5.":                                                               on(effects.ClassLevel, "paladin", steps(dice("2d8"))),
		"The damage equals the Sneak Attack column of the Rogue table.":                                            on(effects.TableColumn, "rogue", effects.Column{Name: "Sneak Attack"}),
		"The damage increases by 1d6 for each Charge spent above 1.":                                               on(effects.Charges, "", effects.Linear{Base: 1, Per: dice("1d6")}),
		"The damage becomes 3d6 with 5 Charges.":                                                                   on(effects.Charges, "", steps(dice("3d6"))),
		"Cantrip Upgrade: the damage increases by 1d10 for each level above 4.":                                    on(effects.CharacterLevel, "", effects.Linear{Base: 4, Per: dice("1d10")}),
		"The damage increases by 1d8 for each Cleric level above 2.":                                               on(effects.ClassLevel, "cleric", effects.Linear{Base: 2, Per: dice("1d8")}),
		"Using a Higher-Level Spell Slot: the damage becomes 9d6 at slot level 5, 10d6 at slot level 11.":          on(effects.SlotLevel, "", steps(dice("9d6"), dice("10d6"))),
		"Using a Higher-Level Spell Slot: the size in feet increases by 5 for each slot level above 2.":            {Part: 0, Field: "size_ft", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 2, Per: n(5)}},
		"Using a Higher-Level Spell Slot: the hit points is 30 + 10 * (level - 3), where level is the slot level.": {Part: 2, Field: "amount", Axis: effects.SlotLevel, Rule: effects.Formula{Expr: "30 + 10 * (level - 3)"}},
		"Cantrip Upgrade: the hit points is 11 + level, where level is the character level.":                       {Part: 2, Field: "amount", Axis: effects.CharacterLevel, Rule: effects.Formula{Expr: "11 + level"}},
		"The hit points is level, where level is the Druid level.":                                                 {Part: 2, Field: "amount", Axis: effects.ClassLevel, Class: "druid", Rule: effects.Formula{Expr: "level"}},
		"The hit points is 2 * level, where level is the number of Charges spent.":                                 {Part: 2, Field: "amount", Axis: effects.Charges, Rule: effects.Formula{Expr: "2 * level"}},
		"Using a Higher-Level Spell Slot: the duration becomes 8 at slot level 5, 24 at slot level 11.":            act("duration", steps(n(8), n(24))),
		"Using a Higher-Level Spell Slot: the duration unit becomes hours at slot level 5.":                        act("duration_unit", steps(dice("hours"))),
		"Using a Higher-Level Spell Slot: the concentration becomes off at slot level 5, on at slot level 11.":     act("concentration", steps(effects.Value{Flag: false}, effects.Value{Flag: true})),
		"Using a Higher-Level Spell Slot: the shade increases by 1d6 for each slot level above 1.":                 {Part: 2, Field: "shade", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: dice("1d6")}},
		"Using a Higher-Level Spell Slot: the range increases by 1d6 for each slot level above 1.":                 act("range", effects.Linear{Base: 1, Per: dice("1d6")}),
		"Using a Higher-Level Spell Slot: the size increases by 1d6 for each slot level above 1.":                  {Part: 7, Field: "size", Axis: effects.SlotLevel, Rule: effects.Linear{Base: 1, Per: dice("1d6")}},
	} {
		d := effects.Definition{Parts: []effects.Part{
			effects.Area{Shape: hex.SphereArea, SizeFt: 20}, effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire"}, effects.TempHP{Amount: 30},
		}, Scaling: []effects.Scaling{s}}
		if got := d.Text(); got[len(got)-1] != want {
			t.Errorf("reads %q\nwant  %q", got[len(got)-1], want)
		}
	}
}

// The lowest slot an Effect is cast with is the least base of its linear slot rules, else 1; an Effect
// that does not scale by slot has none.
func TestTheLowestSlotAnEffectIsCastWith(t *testing.T) {
	t.Parallel()
	by := func(a effects.Axis, r effects.Rule) effects.Scaling {
		return effects.Scaling{Part: 0, Field: "dice", Axis: a, Class: "", Rule: r}
	}
	linear := func(base int) effects.Rule { return effects.Linear{Base: base, Per: effects.Value{Text: "1d6"}} }
	table := effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "2d6"}}}}
	for want, sc := range map[[2]int][]effects.Scaling{
		{0, 0}: nil,
		{0, 1}: {by(effects.CharacterLevel, linear(4))},
		{3, 2}: {by(effects.SlotLevel, linear(3))},
		{2, 2}: {by(effects.SlotLevel, linear(3)), by(effects.SlotLevel, linear(2)), by(effects.CharacterLevel, linear(1))},
		{2, 3}: {by(effects.SlotLevel, linear(2)), by(effects.SlotLevel, linear(4))},
		{1, 4}: {by(effects.SlotLevel, linear(5)), by(effects.SlotLevel, table)},
		{1, 5}: {by(effects.SlotLevel, table)},
	} {
		lowest, ok := (effects.Definition{Scaling: sc}).LowestSlot()
		if lowest != want[0] || ok != (want[0] > 0) {
			t.Errorf("case %d: lowest slot = %d %v", want[1], lowest, ok)
		}
	}
}

// An area Effect is laid out as it is activated: its template and damage grown before it is aimed.
func TestAnAreaIsLaidOutAtItsLevel(t *testing.T) {
	t.Parallel()
	cat := effects.Catalog{"burst": {Name: "Burst", Parts: []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 10, RangeFt: 60},
		effects.SaveDamage{Ability: "dexterity", Dice: "2d6", Type: "fire", Half: true},
	}, Scaling: []effects.Scaling{
		{Part: 0, Field: "size_ft", Axis: effects.SlotLevel, Class: "", Rule: effects.Linear{Base: 2, Per: effects.Value{Number: 5}}},
		{Part: 1, Field: "dice", Axis: effects.SlotLevel, Class: "", Rule: effects.Linear{Base: 2, Per: effects.Value{Text: "1d6"}}},
	}}}
	if a, ok := cat.AreaAt("burst", effects.Level{Slot: 4}); !ok || a.Area.SizeFt != 20 || a.Damage.Dice != "4d6" {
		t.Fatalf("at slot 4 = %+v", a)
	}
	if a, _ := cat.AreaOf("burst"); a.Area.SizeFt != 10 || a.Damage.Dice != "2d6" {
		t.Fatalf("as written = %+v", a)
	}
}

// The field a Scaling grows is the one its Part's kind, or the Activation, describes.
func TestAScalingNamesAFieldThatIsDescribed(t *testing.T) {
	t.Parallel()
	d := effects.Definition{Parts: []effects.Part{effects.Area{Shape: hex.SphereArea, SizeFt: 10}, effects.TempHP{Amount: 3}}}
	for want, s := range map[string]effects.Scaling{
		"Size in feet":  {Part: 0, Field: "size_ft"},
		"Hit points":    {Part: 1, Field: "amount"},
		"Concentration": {Part: effects.OnActivation, Field: "concentration"},
	} {
		if f, ok := d.Scaled(s); !ok || f.Label != want {
			t.Errorf("%+v names %+v %v", s, f, ok)
		}
	}
	for _, s := range []effects.Scaling{{Part: 0, Field: "amount"}, {Part: 2, Field: "dice"}, {Part: effects.OnActivation, Field: "size_ft"}} {
		if f, ok := d.Scaled(s); ok || f.Name != s.Field || f.Type != "" {
			t.Errorf("%+v names %+v", s, f)
		}
	}
}

// The six abilities, in the order the rules list them, are what a repeated save may name.
func TestTheSixAbilities(t *testing.T) {
	t.Parallel()
	want := []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
	if got := effects.Abilities(); !reflect.DeepEqual(got, want) {
		t.Fatalf("abilities = %v", got)
	}
}
