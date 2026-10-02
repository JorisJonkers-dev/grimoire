package features_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

func TestScalesReadTheHighestStepReached(t *testing.T) {
	t.Parallel()
	sneak := features.Scale[string]{}
	for lvl, n := 1, 1; lvl <= 19; lvl, n = lvl+2, n+1 {
		sneak = append(sneak, features.Step[string]{Level: lvl, Value: string(rune('0'+n%10)) + "d6"})
	}
	sneak[9].Value = "10d6"
	for level, want := range map[int]string{1: "1d6", 2: "1d6", 3: "2d6", 4: "2d6", 10: "5d6", 18: "9d6", 19: "10d6", 20: "10d6"} {
		if got, ok := sneak.At(level); !ok || got != want {
			t.Errorf("Sneak Attack at %d = %q %v, want %q", level, got, ok, want)
		}
	}
	if got, ok := sneak.At(0); ok || got != "" {
		t.Errorf("before level 1 = %q %v", got, ok)
	}
	shuffled := features.Scale[int]{{Level: 6, Value: 4}, {Level: 1, Value: 2}, {Level: 3, Value: 3}}
	if got, _ := shuffled.At(5); got != 3 {
		t.Errorf("steps out of order = %d", got)
	}
	if got, _ := (features.Scale[int]{{Level: 2, Value: 1}, {Level: 2, Value: 9}}).At(2); got != 1 {
		t.Errorf("of two steps at one level the first counts = %d", got)
	}
}

func rage() features.Resource {
	return features.Resource{
		Slug: "rage", Name: "Rage", Owner: features.Owner{Kind: "class", Slug: "barbarian"}, Basis: features.ByTable, Multiplier: 0, Ability: "", FromLevel: 1,
		Table: features.Scale[int]{{Level: 1, Value: 2}, {Level: 3, Value: 3}, {Level: 6, Value: 4}, {Level: 12, Value: 5}, {Level: 17, Value: 6}},
		Die:   nil,
		Recharges: []features.Recharge{
			{On: features.ShortRest, FromLevel: 1, Amount: 1, RollAtLeast: 0},
			{On: features.LongRest, FromLevel: 1, Amount: features.All, RollAtLeast: 0},
		},
	}
}

func focus() features.Resource {
	return features.Resource{
		Slug: "focus-points", Name: "Focus Points", Owner: features.Owner{Kind: "class", Slug: "monk"}, Basis: features.ByClassLevel, Multiplier: 1, Ability: "", FromLevel: 2,
		Table: nil, Die: nil,
		Recharges: []features.Recharge{
			{On: features.ShortRest, FromLevel: 2, Amount: features.All, RollAtLeast: 0},
			{On: features.LongRest, FromLevel: 2, Amount: features.All, RollAtLeast: 0},
		},
	}
}

func TestRageUses(t *testing.T) {
	t.Parallel()
	r := rage()
	for level, want := range map[int]int{1: 2, 2: 2, 3: 3, 5: 3, 6: 4, 11: 4, 12: 5, 16: 5, 17: 6, 20: 6} {
		if got := r.Max(features.Stats{Level: level, AbilityMod: 0, Proficiency: 2}); got != want {
			t.Errorf("Rage uses at %d = %d, want %d", level, got, want)
		}
	}
	at6 := features.Stats{Level: 6, AbilityMod: 0, Proficiency: 3}
	if got := r.Regain(features.ShortRest, at6, 0, 0); got != 1 {
		t.Errorf("a Short Rest gives back one Rage = %d", got)
	}
	if got := r.Regain(features.ShortRest, at6, 0, 4); got != 4 {
		t.Errorf("never above the maximum = %d", got)
	}
	if got := r.Regain(features.LongRest, at6, 0, 1); got != 4 {
		t.Errorf("a Long Rest gives back every Rage = %d", got)
	}
	if got := r.Regain(features.Initiative, at6, 0, 1); got != 1 {
		t.Errorf("rolling initiative gives nothing back = %d", got)
	}
}

func TestFocusPointsRecharge(t *testing.T) {
	t.Parallel()
	f := focus()
	if got := f.Max(features.Stats{Level: 1, AbilityMod: 3, Proficiency: 2}); got != 0 {
		t.Errorf("a level 1 monk has no Focus = %d", got)
	}
	for _, level := range []int{2, 7, 20} {
		s := features.Stats{Level: level, AbilityMod: 3, Proficiency: 4}
		if got := f.Max(s); got != level {
			t.Errorf("Focus at %d = %d", level, got)
		}
		if got := f.Regain(features.ShortRest, s, 0, 0); got != level {
			t.Errorf("a Short Rest refills Focus at %d = %d", level, got)
		}
	}
	if got := f.Regain(features.ShortRest, features.Stats{Level: 1, AbilityMod: 3, Proficiency: 2}, 0, 0); got != 0 {
		t.Errorf("no Focus to regain before level 2 = %d", got)
	}
}

func TestOtherBasesAndRecharges(t *testing.T) {
	t.Parallel()
	inspiration := features.Resource{
		Slug: "bardic-inspiration", Name: "Bardic Inspiration", Owner: features.Owner{Kind: "class", Slug: "bard"}, Basis: features.ByAbility, Multiplier: 0, Ability: "charisma", FromLevel: 1, Table: nil,
		Die: features.Scale[string]{{Level: 1, Value: "d6"}, {Level: 5, Value: "d8"}, {Level: 10, Value: "d10"}, {Level: 15, Value: "d12"}},
		Recharges: []features.Recharge{
			{On: features.LongRest, FromLevel: 1, Amount: features.All, RollAtLeast: 0},
			{On: features.ShortRest, FromLevel: 5, Amount: features.All, RollAtLeast: 0},
		},
	}
	weak, strong := features.Stats{Level: 4, AbilityMod: -1, Proficiency: 2}, features.Stats{Level: 5, AbilityMod: 4, Proficiency: 3}
	if inspiration.Max(weak) != 1 || inspiration.Max(strong) != 4 {
		t.Errorf("Bardic Inspiration uses = %d, %d", inspiration.Max(weak), inspiration.Max(strong))
	}
	if d, _ := inspiration.DieAt(strong.Level); d != "d8" {
		t.Errorf("die at 5 = %s", d)
	}
	if _, ok := rage().DieAt(5); ok {
		t.Error("Rage has no die")
	}
	if inspiration.Regain(features.ShortRest, weak, 0, 0) != 0 || inspiration.Regain(features.ShortRest, strong, 0, 0) != 4 {
		t.Error("Font of Inspiration starts at level 5")
	}
	lay := features.Resource{
		Slug: "lay-on-hands", Name: "Lay On Hands", Owner: features.Owner{Kind: "class", Slug: "paladin"}, Basis: features.ByClassLevel, Multiplier: 5, Ability: "", FromLevel: 1, Table: nil, Die: nil,
		Recharges: []features.Recharge{{On: features.LongRest, FromLevel: 1, Amount: features.All, RollAtLeast: 0}},
	}
	if got := lay.Max(features.Stats{Level: 3, AbilityMod: 0, Proficiency: 2}); got != 15 {
		t.Errorf("Lay on Hands pool = %d", got)
	}
	prof := features.Resource{
		Slug: "x", Name: "X", Owner: features.Owner{Kind: "", Slug: ""}, Basis: features.ByProficiency, Multiplier: 0, Ability: "", FromLevel: 1, Table: nil, Die: nil,
		Recharges: []features.Recharge{{On: features.RechargeRoll, FromLevel: 1, Amount: features.All, RollAtLeast: 5}},
	}
	s := features.Stats{Level: 9, AbilityMod: 0, Proficiency: 4}
	if prof.Max(s) != 4 || prof.Regain(features.RechargeRoll, s, 4, 0) != 0 || prof.Regain(features.RechargeRoll, s, 5, 0) != 4 {
		t.Error("proficiency maximum and a recharge on 5–6")
	}
	if got := (features.Resource{Basis: "nonsense", FromLevel: 1}).Max(s); got != 0 {
		t.Errorf("an unknown basis grants nothing = %d", got)
	}
}

func TestChoicesArriveAtTheirLevel(t *testing.T) {
	t.Parallel()
	cs := []features.Choice{
		{Slug: "fighting-style", Name: "Fighting Style", Level: 1, Count: 1, Pool: features.FeatCategory, From: "fighting-style"},
		{Slug: "subclass", Name: "Subclass", Level: 3, Count: 1, Pool: features.Subclass, From: "fighter"},
		{Slug: "asi-4", Name: "Ability Score Improvement", Level: 4, Count: 1, Pool: features.FeatCategory, From: "general"},
	}
	if got := features.ChoicesAt(cs, 3); !reflect.DeepEqual(got, cs[1:2]) {
		t.Errorf("at 3 = %+v", got)
	}
	if got := features.ChoicesAt(cs, 2); got != nil {
		t.Errorf("at 2 = %+v", got)
	}
}

func TestPrerequisites(t *testing.T) {
	t.Parallel()
	grappler := []features.Requirement{
		{Kind: features.MinLevel, Ability: "", Minimum: 4, Slug: "", Group: 0},
		{Kind: features.MinAbility, Ability: "strength", Minimum: 13, Slug: "", Group: 1},
		{Kind: features.MinAbility, Ability: "dexterity", Minimum: 13, Slug: "", Group: 1},
	}
	nimble := features.Candidate{Level: 4, Abilities: map[string]int{"strength": 8, "dexterity": 13}, Spellcasting: false, Feats: nil, Features: nil}
	if got := features.Unmet(grappler, nimble); got != nil {
		t.Errorf("a nimble level 4 meets Grappler = %v", got)
	}
	weak := features.Candidate{Level: 3, Abilities: map[string]int{"strength": 8, "dexterity": 12}, Spellcasting: false, Feats: nil, Features: nil}
	if got := features.Unmet(grappler, weak); !reflect.DeepEqual(got, []string{"Level 4+", "Strength 13+ or Dexterity 13+"}) {
		t.Errorf("unmet = %v", got)
	}
	recall := []features.Requirement{
		{Kind: features.MinLevel, Ability: "", Minimum: 19, Slug: "", Group: 0},
		{Kind: features.Spellcasting, Ability: "", Minimum: 0, Slug: "", Group: 1},
		{Kind: features.HasFeat, Ability: "", Minimum: 0, Slug: "alert", Group: 2},
		{Kind: features.HasFeature, Ability: "", Minimum: 0, Slug: "fighting-style", Group: 3},
	}
	caster := features.Candidate{Level: 19, Abilities: nil, Spellcasting: true, Feats: []string{"alert"}, Features: []string{"fighting-style"}}
	if got := features.Unmet(recall, caster); got != nil {
		t.Errorf("caster = %v", got)
	}
	plain := features.Candidate{Level: 19, Abilities: nil, Spellcasting: false, Feats: nil, Features: nil}
	if got := features.Unmet(recall, plain); !reflect.DeepEqual(got, []string{"Spellcasting", "the alert feat", "the fighting-style feature"}) {
		t.Errorf("plain = %v", got)
	}
	if got := features.Unmet([]features.Requirement{{Kind: "nonsense", Group: 0}}, plain); !reflect.DeepEqual(got, []string{"nonsense"}) {
		t.Errorf("an unknown requirement is never met = %v", got)
	}
}

func TestCatalogFindsWhatAnOwnerGrants(t *testing.T) {
	t.Parallel()
	barbarian := features.Owner{Kind: "class", Slug: "barbarian"}
	cat := features.Catalog{
		Resources:     map[string]features.Resource{"rage": rage(), "focus-points": focus(), "brutal": {Slug: "brutal", Owner: barbarian}},
		Scales:        nil,
		Choices:       nil,
		Prerequisites: nil,
	}
	got := cat.ResourcesOf(barbarian)
	if len(got) != 2 || got[0].Slug != "brutal" || got[1].Slug != "rage" {
		t.Errorf("the barbarian's resources = %+v", got)
	}
}
