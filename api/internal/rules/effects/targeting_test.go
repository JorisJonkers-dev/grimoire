package effects_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func foe(id string, q int) effects.Candidate {
	return effects.Candidate{ID: id, At: hex.Coord{Q: q, R: 0}, Self: false, Ally: false, Object: false, Type: "humanoid", Seen: true}
}

func friend(id string, q int) effects.Candidate {
	c := foe(id, q)
	c.Ally = true
	return c
}

// An Activation offers only legal targets: the count, range, side, creature type, sight, willingness
// and spread its Targeting states; each illegal pick is refused with the reason.
func TestTargetingRefusesIllegalPicks(t *testing.T) {
	t.Parallel()
	bless := effects.Targeting{Kind: effects.TargetCreatures, Count: 3, RangeFt: 30, Side: effects.AnySide, Types: nil, Seen: false, Willing: false, SpreadFt: 0}
	caster := hex.Coord{Q: 0, R: 0}
	cases := []struct {
		name  string
		t     func(effects.Targeting) effects.Targeting
		picks []effects.Candidate
		want  []effects.Refusal
	}{
		{"legal", nil, []effects.Candidate{friend("a", 1), friend("b", 6), foe("c", 2)}, nil},
		{"none chosen", nil, nil, []effects.Refusal{{Pick: -1, Reason: "Choose a target."}}},
		{"too many", nil, []effects.Candidate{friend("a", 1), friend("b", 2), friend("c", 3), friend("d", 4)}, []effects.Refusal{{Pick: -1, Reason: "Choose at most 3 targets."}}},
		{"one by default", func(t effects.Targeting) effects.Targeting { t.Count = 0; return t }, []effects.Candidate{friend("a", 1), friend("b", 2)}, []effects.Refusal{{Pick: -1, Reason: "Choose at most 1 target."}}},
		{"twice", nil, []effects.Candidate{friend("a", 1), friend("a", 1)}, []effects.Refusal{{Pick: 1, Reason: "Choose each target once."}}},
		{"out of range", nil, []effects.Candidate{friend("a", 7)}, []effects.Refusal{{Pick: 0, Reason: "Out of range: the range is 30 feet."}}},
		{"touch reaches the next hex", func(t effects.Targeting) effects.Targeting { t.RangeFt = 0; return t }, []effects.Candidate{friend("a", 1), friend("b", 2)}, []effects.Refusal{{Pick: 1, Reason: "Out of range: the range is touch."}}},
		{"an object", nil, []effects.Candidate{{ID: "door", At: hex.Coord{Q: 1, R: 0}, Object: true}}, []effects.Refusal{{Pick: 0, Reason: "Choose a creature, not an object."}}},
		{"allies only", func(t effects.Targeting) effects.Targeting { t.Side = effects.Allies; return t }, []effects.Candidate{friend("a", 1), foe("b", 1)}, []effects.Refusal{{Pick: 1, Reason: "Choose an ally."}}},
		{"enemies only", func(t effects.Targeting) effects.Targeting { t.Side = effects.Enemies; return t }, []effects.Candidate{friend("a", 1), foe("b", 1)}, []effects.Refusal{{Pick: 0, Reason: "Choose an enemy."}}},
		{"creature type", func(t effects.Targeting) effects.Targeting { t.Types = []string{"undead", "fiend"}; return t }, []effects.Candidate{friend("a", 1)}, []effects.Refusal{{Pick: 0, Reason: "Choose an undead or a fiend."}}},
		{"seen", func(t effects.Targeting) effects.Targeting { t.Seen = true; return t }, []effects.Candidate{{ID: "a", At: hex.Coord{Q: 1, R: 0}, Seen: false}}, []effects.Refusal{{Pick: 0, Reason: "Choose a creature you can see."}}},
		{"willing is the same side", func(t effects.Targeting) effects.Targeting { t.Willing = true; return t }, []effects.Candidate{friend("a", 1), foe("b", 1)}, []effects.Refusal{{Pick: 1, Reason: "Choose a willing creature."}}},
		{"spread from the first", func(t effects.Targeting) effects.Targeting { t.SpreadFt = 10; return t }, []effects.Candidate{friend("a", 1), friend("b", 3), friend("c", 4)}, []effects.Refusal{{Pick: 2, Reason: "Choose a target within 10 feet of the first."}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tg := bless
			if c.t != nil {
				tg = c.t(tg)
			}
			if got := tg.Refusals(caster, c.picks); !reflect.DeepEqual(got, c.want) {
				t.Errorf("%s: %+v", c.name, got)
			}
		})
	}
}

// Self targets only the one activating; a point is one place in range; an area picks its own targets.
func TestOtherTargetingKinds(t *testing.T) {
	t.Parallel()
	caster := hex.Coord{Q: 0, R: 0}
	self := effects.Targeting{Kind: effects.TargetSelf}
	me := effects.Candidate{ID: "me", At: caster, Self: true, Ally: true, Seen: true}
	if got := self.Refusals(caster, []effects.Candidate{me}); got != nil {
		t.Errorf("self on self: %+v", got)
	}
	if got := self.Refusals(caster, nil); got != nil {
		t.Errorf("self with no pick: %+v", got)
	}
	if got := self.Refusals(caster, []effects.Candidate{friend("a", 1)}); !reflect.DeepEqual(got, []effects.Refusal{{Pick: 0, Reason: "This affects only you."}}) {
		t.Errorf("self on another: %+v", got)
	}
	door := effects.Candidate{ID: "door", At: hex.Coord{Q: 2, R: 0}, Object: true}
	object := effects.Targeting{Kind: effects.TargetObject, Count: 1, RangeFt: 60}
	if got := object.Refusals(caster, []effects.Candidate{door}); got != nil {
		t.Errorf("an object: %+v", got)
	}
	if got := object.Refusals(caster, []effects.Candidate{friend("a", 1)}); !reflect.DeepEqual(got, []effects.Refusal{{Pick: 0, Reason: "Choose an object, not a creature."}}) {
		t.Errorf("a creature as an object: %+v", got)
	}
	point := effects.Targeting{Kind: effects.TargetPoint, RangeFt: 60}
	if got := point.Refusals(caster, []effects.Candidate{{ID: "", At: hex.Coord{Q: 12, R: 0}}}); got != nil {
		t.Errorf("a point in range: %+v", got)
	}
	if got := point.Refusals(caster, []effects.Candidate{{ID: "", At: hex.Coord{Q: 13, R: 0}}}); !reflect.DeepEqual(got, []effects.Refusal{{Pick: 0, Reason: "Out of range: the range is 60 feet."}}) {
		t.Errorf("a point out of range: %+v", got)
	}
	area := effects.Targeting{Kind: effects.TargetArea, RangeFt: 150}
	if got := area.Refusals(caster, []effects.Candidate{friend("a", 1)}); !reflect.DeepEqual(got, []effects.Refusal{{Pick: -1, Reason: "The area picks its own targets."}}) {
		t.Errorf("an area with picks: %+v", got)
	}
	if got := area.Refusals(caster, nil); got != nil {
		t.Errorf("an area: %+v", got)
	}
	if got := effects.TargetKinds(); !reflect.DeepEqual(got, []string{"self", "creatures", "object", "point", "area"}) {
		t.Errorf("kinds = %v", got)
	}
	if got := (effects.Targeting{}).Refusals(caster, []effects.Candidate{friend("a", 30)}); got != nil {
		t.Errorf("no Targeting stated: %+v", got)
	}
}

// Scaling grows the number of targets as an Activation value: Bless takes one more creature for each
// slot level above 1.
func TestScalingGrowsTheNumberOfTargets(t *testing.T) {
	t.Parallel()
	d := effects.Definition{
		Targeting: effects.Targeting{Kind: effects.TargetCreatures, Count: 3, RangeFt: 30},
		Scaling: []effects.Scaling{{
			Part: effects.OnActivation, Field: "targets", Axis: effects.SlotLevel,
			Rule: effects.Linear{Base: 1, Per: effects.Value{Number: 1}},
		}},
	}
	if got := d.At(effects.Level{Slot: 3}).Targeting.Count; got != 5 {
		t.Fatalf("targets at slot 3 = %d", got)
	}
	want := []string{
		"Range: 30 feet. Targets: up to 3 creatures.",
		"Using a Higher-Level Spell Slot: the number of targets increases by 1 for each slot level above 1.",
	}
	if got := d.Text(); !reflect.DeepEqual(got, want) {
		t.Fatalf("text = %q", got)
	}
	fields, scales := effects.ActivationFields(), effects.ActivationScales()
	f, s := fields[len(fields)-1], scales[len(scales)-1]
	if f.Name != "targets" || f.Type != effects.NumberField || f.Min != 1 || f.Max != 50 || s != (effects.Scalable{Field: "targets", Formula: true}) {
		t.Fatalf("activation fields = %+v, %+v", f, s)
	}
}

// Targeting reads as the rules text a spell prints.
func TestTargetingReadsAsRulesText(t *testing.T) {
	t.Parallel()
	cases := map[string]effects.Targeting{
		"Range: 30 feet. Targets: up to 3 creatures.":                                   {Kind: effects.TargetCreatures, Count: 3, RangeFt: 30},
		"Range: touch. Targets: a willing creature.":                                    {Kind: effects.TargetCreatures, Count: 1, Willing: true},
		"Range: 60 feet. Targets: a creature you can see that is an undead or a fiend.": {Kind: effects.TargetCreatures, RangeFt: 60, Seen: true, Types: []string{"undead", "fiend"}},
		"Range: 30 feet. Targets: up to 2 enemies, each within 10 feet of the first.":   {Kind: effects.TargetCreatures, Count: 2, RangeFt: 30, Side: effects.Enemies, SpreadFt: 10},
		"Range: 30 feet. Targets: an ally.":                                             {Kind: effects.TargetCreatures, RangeFt: 30, Side: effects.Allies},
		"Range: self.":                                                                  {Kind: effects.TargetSelf},
		"Range: 60 feet. Targets: an object.":                                           {Kind: effects.TargetObject, RangeFt: 60},
		"Range: 120 feet. Targets: a point.":                                            {Kind: effects.TargetPoint, RangeFt: 120},
		"Range: 150 feet.":                                                              {Kind: effects.TargetArea, RangeFt: 150},
		"":                                                                              {},
	}
	for want, tg := range cases {
		if got := tg.Text(); got != want {
			t.Errorf("%+v: %q, want %q", tg, got, want)
		}
	}
}

// A Targeting is checked before it is stored: a kind the rules know, and counts and distances in range.
func TestTargetingIsChecked(t *testing.T) {
	t.Parallel()
	ok := effects.Targeting{Kind: effects.TargetCreatures, Count: 50, RangeFt: 5280, Side: effects.Enemies, Types: make([]string, 14), SpreadFt: 5280}
	for i := range ok.Types {
		ok.Types[i] = "undead"
	}
	if err := ok.Check(); err != nil {
		t.Fatalf("the limits: %v", err)
	}
	if err := (effects.Targeting{Kind: effects.TargetSelf}).Check(); err != nil {
		t.Fatalf("self: %v", err)
	}
	for want, change := range map[string]func(*effects.Targeting){
		"kind: choose one of the options":      func(t *effects.Targeting) { t.Kind = "everyone" },
		"count: runs from 0 to 50":             func(t *effects.Targeting) { t.Count = 51 },
		"count: runs from 0 to 50 ":            func(t *effects.Targeting) { t.Count = -1 },
		"range_ft: runs from 0 to 5280":        func(t *effects.Targeting) { t.RangeFt = 5281 },
		"range_ft: runs from 0 to 5280 ":       func(t *effects.Targeting) { t.RangeFt = -1 },
		"side: choose one of the options":      func(t *effects.Targeting) { t.Side = "neutral" },
		"creature_types: name up to 14 types":  func(t *effects.Targeting) { t.Types = append(t.Types, "fey") },
		"creature_types: name up to 14 types ": func(t *effects.Targeting) { t.Types[0] = "" },
		"spread_ft: runs from 0 to 5280":       func(t *effects.Targeting) { t.SpreadFt = 5281 },
		"spread_ft: runs from 0 to 5280 ":      func(t *effects.Targeting) { t.SpreadFt = -1 },
	} {
		t.Run(want, func(t *testing.T) {
			bad := ok
			bad.Types = append([]string{}, ok.Types...)
			change(&bad)
			if err := bad.Check(); err == nil || err.Error() != strings.TrimSpace(want) {
				t.Errorf("want %q, got %v", want, err)
			}
		})
	}
}
