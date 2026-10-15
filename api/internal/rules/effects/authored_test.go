package effects_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func authored(t *testing.T, raw string) []effects.Authored {
	t.Helper()
	var out []effects.Authored
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Parts as an author writes them, a kind's slug, its values and the Parts beneath it in named groups,
// decode through each kind's own definition, Parts beneath first.
func TestAuthoredPartsDecodeThroughTheirKinds(t *testing.T) {
	t.Parallel()
	in := authored(t, `[
		{"kind":"save_damage","values":{"ability":{"text":"dexterity"},"dice":{"text":"8d6"},"damage_type":{"text":"fire"},"half":{"flag":true}},"groups":[]},
		{"kind":"branch","values":{"when":{"text":"fails_by"},"n":{"number":5}},"groups":[
			{"name":"then","parts":[{"kind":"save_condition","values":{"ability":{"text":"dexterity"},"condition":{"text":"prone"}},"groups":[]}]}]},
		{"kind":"choice","values":{},"groups":[
			{"name":"Enlarge","parts":[{"kind":"reveal","values":{"qualities":{"texts":["invisible","hidden"]}},"groups":[]}]},
			{"name":"Reduce","parts":[{"kind":"manual","values":{"instruction":{"text":"Shrink."}},"groups":[]}]}]},
		{"kind":"incapacitated"}]`)
	got, err := effects.Decode(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []effects.Part{
		effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
		effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5, Type: ""}, Then: []effects.Part{effects.SaveCondition{Ability: "dexterity", Slug: "prone"}}},
		effects.Choice{Modes: []effects.Group{
			{Name: "Enlarge", Parts: []effects.Part{effects.Reveal{Qualities: []string{"invisible", "hidden"}}}},
			{Name: "Reduce", Parts: []effects.Part{effects.Manual{Instruction: "Shrink."}}},
		}},
		effects.Incapacitated{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded =\n%+v\nwant\n%+v", got, want)
	}
	if out, _ := json.Marshal(in[0].Values); string(out) != `{"ability":{"text":"dexterity"},"damage_type":{"text":"fire"},"dice":{"text":"8d6"},"half":{"flag":true}}` {
		t.Fatalf("values write back as %s", out)
	}
	if out, _ := json.Marshal(in[1].Values["n"]); string(out) != `{"number":5}` {
		t.Fatalf("a number writes back as %s", out)
	}
	if none, err := effects.Decode(nil); err != nil || len(none) != 0 {
		t.Fatalf("no Parts = %v %v", none, err)
	}
	if k, ok := effects.KindOf("save_damage"); !ok || k.Describe().Label != "Save for damage" {
		t.Fatalf("save_damage = %v %v", k, ok)
	}
	if k, ok := effects.KindOf("song"); ok || k != nil {
		t.Fatalf("a kind the rules do not know = %v %v", k, ok)
	}
}

// What an author wrote that the definitions refuse names where it sits and why.
func TestAuthoredPartsThatDoNotDecode(t *testing.T) {
	t.Parallel()
	prone := `{"kind":"save_condition","values":{"ability":{"text":"dexterity"},"condition":{"text":"prone"}},"groups":[]}`
	nest := func(depth int) string {
		out := prone
		for range depth {
			out = `{"kind":"branch","values":{"when":{"text":"first_each_turn"}},"groups":[{"name":"then","parts":[` + out + `]}]}`
		}
		return "[" + out + "]"
	}
	for raw, want := range map[string]string{
		`[` + prone + `,{"kind":"song","values":{},"groups":[]}]`:                                                                                                                                 "part 2: kind: choose a kind the rules know",
		`[{"kind":"incapacitated","values":{"colour":{"text":"red"}},"groups":[]}]`:                                                                                                               "part 1: colour: Incapacitated has no such field",
		`[{"kind":"light","values":{"bright_ft":{"number":5}},"groups":[{"name":"then"}]}]`:                                                                                                       "part 1: groups: Light holds no group named then",
		`[{"kind":"branch","values":{"when":{"text":"first_each_turn"}},"groups":[{"name":"else","parts":[` + prone + `]}]}]`:                                                                     "part 1: groups: Branch holds no group named else",
		`[{"kind":"choice","values":{},"groups":[{"name":"A","parts":[` + prone + `]},{"name":"A","parts":[` + prone + `]}]}]`:                                                                    "part 1: groups: name each group once",
		`[{"kind":"branch","values":{"when":{"text":"first_each_turn"}},"groups":[{"name":"then","parts":[` + prone + `,{"kind":"save_damage","values":{"dice":{"text":"8d7"}},"groups":[]}]}]}]`: "part 1: then: part 2: ability: choose one of the options",
		nest(5): "part 1: then: part 1: then: part 1: then: part 1: then: part 1: groups: nest Parts at most 4 deep",
	} {
		_, err := effects.Decode(authored(t, raw))
		if err == nil || err.Error() != want {
			t.Errorf("%s\n= %v\nwant %s", raw, err, want)
		}
	}
	if _, err := effects.Decode(authored(t, nest(4))); err != nil {
		t.Fatalf("Parts 4 deep: %v", err)
	}
	_, err := effects.Decode(authored(t, `[{"kind":"light","values":{"bright_ft":{"number":500}},"groups":[]}]`))
	var refused effects.ValueError
	if !errors.As(err, &refused) || refused.Field != "bright_ft" || !strings.Contains(refused.Reason, "runs from 5 to 120") {
		t.Fatalf("the kind's own refusal is kept = %v", err)
	}
}

// An authored Part with no values or groups writes them as empty, never null, as the wire requires.
func TestAuthoredPartsWriteEmptyAsEmpty(t *testing.T) {
	t.Parallel()
	bare := effects.Authored{Kind: "branch", Values: nil, Groups: []effects.AuthoredGroup{{Name: "then", Parts: nil}}}
	out, err := json.Marshal([]effects.Authored{bare, {Kind: "immobile", Values: nil, Groups: nil}})
	if err != nil || string(out) != `[{"kind":"branch","values":{},"groups":[{"name":"then","parts":[]}]},{"kind":"immobile","values":{},"groups":[]}]` {
		t.Fatalf("written as %s %v", out, err)
	}
}
