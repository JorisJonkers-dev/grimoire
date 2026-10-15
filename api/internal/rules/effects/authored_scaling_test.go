package effects_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func dice(s string) *effects.Value { return &effects.Value{Text: s} }

// Scaling written on a Part decodes against the Part's kind into rules over its fields, the Parts
// counted depth first from where the Effect's own Parts begin.
func TestAuthoredScalingDecodesAgainstItsPart(t *testing.T) {
	t.Parallel()
	in := []effects.Authored{
		{Kind: "save_damage", Values: effects.Values{"ability": {Text: "dexterity"}, "dice": {Text: "8d6"}, "damage_type": {Text: "fire"}}, Scaling: []effects.AuthoredScaling{
			{Field: "dice", Axis: "slot_level", Rule: "linear", Base: 3, Per: dice("1d6")},
			{Field: "dice", Axis: "character_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 5, Value: effects.Value{Text: "2d10"}}, {At: 11, Value: effects.Value{Text: "3d10"}}}},
			{Field: "dice", Axis: "table_column", Class: "rogue", Rule: "column", Column: "Sneak Attack"},
		}},
		{Kind: "branch", Values: effects.Values{"when": {Text: "hp_at_most"}, "n": {Number: 20}}, Scaling: []effects.AuthoredScaling{
			{Field: "n", Axis: "class_level", Class: "cleric", Rule: "linear", Base: 1, Per: &effects.Value{Number: 10}},
		}, Groups: []effects.AuthoredGroup{{Name: "then", Parts: []effects.Authored{
			{Kind: "temp_hp", Values: effects.Values{"amount": {Number: 5}}, Scaling: []effects.AuthoredScaling{
				{Field: "amount", Axis: "charges", Rule: "formula", Formula: "5 * level"},
				{Field: "amount", Axis: "slot_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 3, Value: effects.Value{Number: 9}}}},
			}},
		}}}},
	}
	got, err := effects.DecodeScaling(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []effects.Scaling{
		{Part: 2, Field: "dice", Axis: effects.SlotLevel, Class: "", Rule: effects.Linear{Base: 3, Per: effects.Value{Text: "1d6"}}},
		{Part: 2, Field: "dice", Axis: effects.CharacterLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "2d10"}}, {At: 11, Value: effects.Value{Text: "3d10"}}}}},
		{Part: 2, Field: "dice", Axis: effects.TableColumn, Class: "rogue", Rule: effects.Column{Name: "Sneak Attack"}},
		{Part: 3, Field: "n", Axis: effects.ClassLevel, Class: "cleric", Rule: effects.Linear{Base: 1, Per: effects.Value{Number: 10}}},
		{Part: 4, Field: "amount", Axis: effects.Charges, Class: "", Rule: effects.Formula{Expr: "5 * level"}},
		{Part: 4, Field: "amount", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 3, Value: effects.Value{Number: 9}}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scaling:\n got %+v\nwant %+v", got, want)
	}
	if none, err := effects.DecodeScaling([]effects.Authored{{Kind: "manual", Values: effects.Values{"instruction": {Text: "x"}}}}, 0); err != nil || none != nil {
		t.Fatalf("no scaling = %+v %v", none, err)
	}
}

// The Activation's own Scaling decodes against its fields.
func TestAuthoredActivationScalingDecodes(t *testing.T) {
	t.Parallel()
	got, err := effects.DecodeActivationScaling([]effects.AuthoredScaling{
		{Field: "concentration", Axis: "slot_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 4, Value: effects.Value{Flag: false}}}},
		{Field: "duration_unit", Axis: "slot_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 5, Value: effects.Value{Text: "hours"}}}},
		{Field: "duration", Axis: "slot_level", Rule: "formula", Formula: "level"},
	})
	want := []effects.Scaling{
		{Part: effects.OnActivation, Field: "concentration", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 4, Value: effects.Value{Flag: false}}}}},
		{Part: effects.OnActivation, Field: "duration_unit", Axis: effects.SlotLevel, Class: "", Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "hours"}}}}},
		{Part: effects.OnActivation, Field: "duration", Axis: effects.SlotLevel, Class: "", Rule: effects.Formula{Expr: "level"}},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("activation scaling = %+v %v", got, err)
	}
}

// Scaling that cannot be right is refused, saying where it sits and which field is wrong.
func TestAuthoredScalingThatDoesNotDecode(t *testing.T) {
	t.Parallel()
	damage := func(s effects.AuthoredScaling) []effects.Authored {
		return []effects.Authored{{Kind: "save_damage", Values: effects.Values{"dice": {Text: "2d6"}}, Scaling: []effects.AuthoredScaling{s}}}
	}
	amount := func(s effects.AuthoredScaling) []effects.Authored {
		return []effects.Authored{{Kind: "temp_hp", Values: effects.Values{"amount": {Number: 5}}, Scaling: []effects.AuthoredScaling{s}}}
	}
	steps := func(st ...effects.AuthoredStep) []effects.AuthoredStep { return st }
	at := func(n int, v effects.Value) effects.AuthoredStep { return effects.AuthoredStep{At: n, Value: v} }
	slot := func(field, rule string) effects.AuthoredScaling {
		return effects.AuthoredScaling{Field: field, Axis: "slot_level", Rule: rule}
	}
	with := func(s effects.AuthoredScaling, f func(*effects.AuthoredScaling)) effects.AuthoredScaling {
		f(&s)
		return s
	}
	many := make([]effects.AuthoredScaling, 11)
	for i := range many {
		many[i] = with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, dice("1d6") })
	}
	for want, in := range map[string][]effects.Authored{
		"part 1: scaling 1: field: choose a field that scales":                 damage(slot("damage_type", "linear")),
		"part 1: scaling 1: axis: choose one of the options":                   damage(effects.AuthoredScaling{Field: "dice", Axis: "moon", Rule: "linear", Base: 1, Per: dice("1d6")}),
		"part 1: scaling 1: rule: choose one of the options":                   damage(slot("dice", "curve")),
		"part 1: scaling 1: class: name the class in up to 80 letters":         damage(effects.AuthoredScaling{Field: "dice", Axis: "class_level", Rule: "linear", Base: 1, Per: dice("1d6")}),
		"part 1: scaling 2: class: name the class in up to 80 letters":         {{Kind: "save_damage", Scaling: []effects.AuthoredScaling{with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, dice("1d6") }), {Field: "dice", Axis: "class_level", Class: strings.Repeat("a", 81), Rule: "linear", Base: 1, Per: dice("1d6")}}}},
		"part 1: scaling 1: base: runs from 0 to 20":                           damage(with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 21, dice("1d6") })),
		"part 1: scaling 1: base: runs from 0 to 20 ":                          damage(with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = -1, dice("1d6") })),
		"part 1: scaling 1: per: write the dice like 1d6":                      damage(with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, dice("lots") })),
		"part 1: scaling 1: per: write the dice like 1d6 ":                     damage(with(slot("dice", "linear"), func(s *effects.AuthoredScaling) { s.Base = 1 })),
		"part 1: scaling 1: per: runs from -1000 to 1000, not 0":               amount(with(slot("amount", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, &effects.Value{Number: 0} })),
		"part 1: scaling 1: per: runs from -1000 to 1000, not 0 ":              amount(with(slot("amount", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, &effects.Value{Number: 1001} })),
		"part 1: scaling 1: per: runs from -1000 to 1000, not 0  ":             amount(with(slot("amount", "linear"), func(s *effects.AuthoredScaling) { s.Base, s.Per = 1, &effects.Value{Number: -1001} })),
		"part 1: scaling 1: steps: give 1 to 20 steps, each at a higher level": damage(slot("dice", "table")),
		"part 1: scaling 1: steps: give 1 to 20 steps, each at a higher level ": damage(with(slot("dice", "table"), func(s *effects.AuthoredScaling) {
			s.Steps = steps(at(5, effects.Value{Text: "2d6"}), at(5, effects.Value{Text: "3d6"}))
		})),
		"part 1: scaling 1: steps: give 1 to 20 steps, each at a higher level  ":  damage(with(slot("dice", "table"), func(s *effects.AuthoredScaling) { s.Steps = steps(at(0, effects.Value{Text: "2d6"})) })),
		"part 1: scaling 1: steps: give 1 to 20 steps, each at a higher level   ": damage(with(slot("dice", "table"), func(s *effects.AuthoredScaling) { s.Steps = steps(at(21, effects.Value{Text: "2d6"})) })),
		"part 1: scaling 1: steps: write the dice like 1d6":                       damage(with(slot("dice", "table"), func(s *effects.AuthoredScaling) { s.Steps = steps(at(5, effects.Value{Text: "2"})) })),
		"part 1: scaling 1: steps: runs from 1 to 500":                            amount(with(slot("amount", "table"), func(s *effects.AuthoredScaling) { s.Steps = steps(at(5, effects.Value{Number: 501})) })),
		"part 1: scaling 1: steps: runs from 1 to 500 ":                           amount(with(slot("amount", "table"), func(s *effects.AuthoredScaling) { s.Steps = steps(at(5, effects.Value{Number: 0})) })),
		"part 1: scaling 1: formula: write a formula of level, like 11 + level":   amount(with(slot("amount", "formula"), func(s *effects.AuthoredScaling) { s.Formula = "level +" })),
		"part 1: scaling 1: formula: write a formula of level, like 11 + level ": amount(with(slot("amount", "formula"), func(s *effects.AuthoredScaling) {
			s.Formula = "1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1"
		})),
		"part 1: scaling 1: formula: write a formula of level, like 11 + level  ":     amount(with(slot("amount", "formula"), func(s *effects.AuthoredScaling) { s.Formula = "level\t+ 1" })),
		"part 1: scaling 1: rule: this field takes no formula":                        damage(with(slot("dice", "formula"), func(s *effects.AuthoredScaling) { s.Formula = "level" })),
		"part 1: scaling 1: rule: a class table's column scales by its column alone":  damage(with(slot("dice", "column"), func(s *effects.AuthoredScaling) { s.Column = "Sneak Attack" })),
		"part 1: scaling 1: rule: a class table's column scales by its column alone ": damage(effects.AuthoredScaling{Field: "dice", Axis: "table_column", Class: "rogue", Rule: "linear", Base: 1, Per: dice("1d6")}),
		"part 1: scaling 1: column: name the column in up to 80 letters":              damage(effects.AuthoredScaling{Field: "dice", Axis: "table_column", Class: "rogue", Rule: "column"}),
		"part 1: scaling 1: column: name the column in up to 80 letters ":             damage(effects.AuthoredScaling{Field: "dice", Axis: "table_column", Class: "rogue", Rule: "column", Column: strings.Repeat("c", 81)}),
		"part 1: scaling: scale a Part by at most 10 rules":                           {{Kind: "save_damage", Scaling: many}},
		"part 1: then: part 1: scaling 1: field: choose a field that scales": {{Kind: "branch", Groups: []effects.AuthoredGroup{{Name: "then", Parts: []effects.Authored{
			{Kind: "manual", Scaling: []effects.AuthoredScaling{slot("instruction", "linear")}},
		}}}}},
		"part 1: kind: choose a kind the rules know": {{Kind: "comet", Scaling: []effects.AuthoredScaling{slot("dice", "linear")}}},
	} {
		_, err := effects.DecodeScaling(in, 0)
		var pe effects.PartError
		if err == nil || !errors.As(err, &pe) || err.Error() != strings.TrimSpace(want) {
			t.Errorf("want %q, got %v", strings.TrimSpace(want), err)
		}
	}
	for want, in := range map[string][]effects.AuthoredScaling{
		"scaling 1: field: choose a field that scales": {{Field: "size_ft", Axis: "slot_level", Rule: "linear", Base: 1, Per: &effects.Value{Number: 5}}},
		"scaling 2: steps: choose one of the options": {
			{Field: "duration", Axis: "slot_level", Rule: "linear", Base: 1, Per: &effects.Value{Number: 5}},
			{Field: "duration_unit", Axis: "slot_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 4, Value: effects.Value{Text: "fortnights"}}}},
		},
		"scaling 1: rule: this field grows only by a table":   {{Field: "concentration", Axis: "slot_level", Rule: "linear", Base: 1, Per: dice("1d6")}},
		"scaling 1: rule: this field grows only by a table ":  {{Field: "duration_unit", Axis: "slot_level", Rule: "linear", Base: 1, Per: dice("1d6")}},
		"scaling 1: rule: this field grows only by a table  ": {{Field: "duration_unit", Axis: "table_column", Class: "bard", Rule: "column", Column: "Song"}},
		"scaling: scale the Activation by at most 10 rules":   make([]effects.AuthoredScaling, 11),
	} {
		_, err := effects.DecodeActivationScaling(in)
		if err == nil || err.Error() != strings.TrimSpace(want) {
			t.Errorf("want %q, got %v", strings.TrimSpace(want), err)
		}
	}
}

// Authored Scaling goes over the wire as written, and a Part without any writes none.
func TestAuthoredScalingOnTheWire(t *testing.T) {
	t.Parallel()
	in := effects.Authored{Kind: "save_damage", Values: effects.Values{"dice": {Text: "2d6"}}, Scaling: []effects.AuthoredScaling{
		{Field: "dice", Axis: "slot_level", Rule: "table", Steps: []effects.AuthoredStep{{At: 2, Value: effects.Value{Text: "3d6"}}}},
	}}
	raw, err := json.Marshal(in)
	if err != nil || string(raw) != `{"kind":"save_damage","values":{"dice":{"text":"2d6"}},"groups":[],"scaling":[{"field":"dice","axis":"slot_level","rule":"table","steps":[{"at":2,"value":{"text":"3d6"}}]}]}` {
		t.Fatalf("written as %s %v", raw, err)
	}
	var back effects.Authored
	if err := json.Unmarshal(raw, &back); err != nil || !reflect.DeepEqual(back, effects.Authored{Kind: in.Kind, Values: in.Values, Groups: []effects.AuthoredGroup{}, Scaling: in.Scaling}) {
		t.Fatalf("read back as %+v %v", back, err)
	}
	if raw, _ := json.Marshal(effects.Authored{Kind: "dispel"}); string(raw) != `{"kind":"dispel","values":{},"groups":[]}` {
		t.Fatalf("no scaling written as %s", raw)
	}
}
