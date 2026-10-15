package effects_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// The kinds a builder shows hint their forms: the order of their fields, the fields drawn together,
// and help where a label is not enough. Every other kind gives none.
func TestKindsHintTheirForms(t *testing.T) {
	t.Parallel()
	none := effects.Hints{Order: nil, Sections: nil, Help: nil}
	want := map[string]effects.Hints{
		"branch": {Order: nil, Sections: []effects.Section{{Label: "Test", Fields: []string{"when", "n", "creature_type"}}}, Help: []effects.Help{
			{Field: "n", Text: "For fails by, how far the save fails; for HP at most, the Hit Points."},
			{Field: "creature_type", Text: "For creature is, such as fey."},
		}},
		"create_surface": {Order: nil, Sections: nil, Help: []effects.Help{{Field: "rounds", Text: "How long the Surface lies."}}},
		"crit_within":    {Order: nil, Sections: nil, Help: []effects.Help{{Field: "feet", Text: "A hit from this close is a Critical Hit."}}},
		"edge": {
			Order:    []string{"advantage", "against", "range", "source_only"},
			Sections: []effects.Section{{Label: "Which attacks", Fields: []string{"against", "range", "source_only"}}},
			Help: []effects.Help{
				{Field: "advantage", Text: "Off: Disadvantage."},
				{Field: "against", Text: "Off: the bearer's own attacks."},
				{Field: "source_only", Text: "Only whoever applied the Effect, as with Vex."},
			},
		},
		"light":         {Order: nil, Sections: nil, Help: []effects.Help{{Field: "dim_ft", Text: "Beyond the bright light; 0 for none."}}},
		"manual":        {Order: nil, Sections: nil, Help: []effects.Help{{Field: "instruction", Text: "What the DM decides when the Effect lands; play waits for it."}}},
		"save_damage":   {Order: []string{"dice", "damage_type", "half", "ability"}, Sections: nil, Help: []effects.Help{{Field: "dice", Text: "Like 8d6."}}},
		"save_edge":     {Order: []string{"mode", "ability"}, Sections: nil, Help: nil},
		"speed_penalty": {Order: nil, Sections: nil, Help: []effects.Help{{Field: "ft", Text: "Several penalties do not add up; the largest counts."}}},
	}
	for _, k := range effects.Kinds() {
		slug := k.Describe().Slug
		hints, ok := want[slug]
		if !ok {
			hints = none
		}
		if got := effects.HintsOf(k); !reflect.DeepEqual(got, hints) {
			t.Errorf("%s hints %+v, want %+v", slug, got, hints)
		}
	}
}

// A hint names only fields its kind describes: each once in the order, in one section at most, and
// with one line of help at most.
func TestHintsNameTheirKindsFields(t *testing.T) {
	t.Parallel()
	for _, k := range effects.Kinds() {
		d, h := k.Describe(), effects.HintsOf(k)
		has := func(name string) bool {
			return slices.ContainsFunc(d.Fields, func(f effects.Field) bool { return f.Name == name })
		}
		seen := map[string]int{}
		named := func(what string, names ...string) {
			for _, n := range names {
				if !has(n) {
					t.Errorf("%s: %s names %q, which it does not describe", d.Slug, what, n)
				}
				seen[what+n]++
				if seen[what+n] > 1 {
					t.Errorf("%s: %s names %q twice", d.Slug, what, n)
				}
			}
		}
		named("order", h.Order...)
		for _, s := range h.Sections {
			if s.Label == "" || len(s.Fields) == 0 {
				t.Errorf("%s: a section without a label or fields", d.Slug)
			}
			named("section", s.Fields...)
		}
		for _, help := range h.Help {
			if help.Text == "" {
				t.Errorf("%s: empty help for %s", d.Slug, help.Field)
			}
			named("help", help.Field)
		}
	}
}
