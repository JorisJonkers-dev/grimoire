package rules_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

func abilitySet(str, dex, con, intl, wis, cha int) map[rules.Ability]int {
	return map[rules.Ability]int{
		rules.Strength: str, rules.Dexterity: dex, rules.Constitution: con,
		rules.Intelligence: intl, rules.Wisdom: wis, rules.Charisma: cha,
	}
}

func TestMulticlassNeedsThirteenInEveryPrimaryAbility(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		s       map[rules.Ability]int
		classes []string
		target  string
		unmet   []string
	}{
		{"fighter by strength into wizard", abilitySet(13, 8, 10, 13, 10, 10), []string{"fighter"}, "wizard", nil},
		{"fighter by dexterity", abilitySet(8, 13, 10, 13, 10, 10), []string{"fighter"}, "wizard", nil},
		{"fighter with neither", abilitySet(12, 12, 10, 13, 10, 10), []string{"fighter"}, "wizard", []string{"Strength 13+ or Dexterity 13+ (fighter)"}},
		{"wizard too low", abilitySet(13, 8, 10, 12, 10, 10), []string{"fighter"}, "wizard", []string{"Intelligence 13+ (wizard)"}},
		{"paladin needs both", abilitySet(13, 8, 10, 10, 10, 12), []string{"fighter"}, "paladin", []string{"Charisma 13+ (paladin)"}},
		{"monk needs both", abilitySet(10, 13, 10, 10, 12, 10), []string{"rogue"}, "monk", []string{"Wisdom 13+ (monk)"}},
		{"every class counted once", abilitySet(10, 10, 10, 10, 10, 13), []string{"bard", "bard"}, "sorcerer", nil},
		{"unknown class has no requirement", abilitySet(3, 3, 3, 3, 3, 3), []string{"homebrew"}, "other", nil},
		{"same class needs nothing", abilitySet(3, 3, 3, 3, 3, 3), []string{"wizard"}, "wizard", nil},
	}
	for _, c := range cases {
		if got := rules.MulticlassUnmet(c.s, c.classes, c.target); !reflect.DeepEqual(got, c.unmet) {
			t.Errorf("%s: %v, want %v", c.name, got, c.unmet)
		}
	}
}

func TestHitPointGain(t *testing.T) {
	t.Parallel()
	cases := []struct{ die, con, roll, want int }{
		{10, 2, 0, 8},
		{8, 0, 0, 5},
		{6, -1, 0, 3},
		{12, 3, 0, 10},
		{10, 2, 1, 3},
		{10, -3, 1, 1},
		{10, 0, 10, 10},
		{6, -5, 0, 1},
	}
	for _, c := range cases {
		if got := rules.HitPointGain(c.die, c.con, c.roll); got != c.want {
			t.Errorf("d%d con %d roll %d = %d, want %d", c.die, c.con, c.roll, got, c.want)
		}
	}
}

func TestAbilityScoreImprovement(t *testing.T) {
	t.Parallel()
	got, err := rules.ImproveAbilities(abilitySet(15, 14, 13, 8, 10, 10), map[rules.Ability]int{rules.Strength: 2})
	if err != nil || got[rules.Strength] != 17 || got[rules.Dexterity] != 14 {
		t.Fatalf("+2 = %v %v", got, err)
	}
	got, err = rules.ImproveAbilities(abilitySet(19, 14, 13, 8, 10, 10), map[rules.Ability]int{rules.Strength: 1, rules.Constitution: 1})
	if err != nil || got[rules.Strength] != 20 || got[rules.Constitution] != 14 {
		t.Fatalf("+1/+1 = %v %v", got, err)
	}
	got, err = rules.ImproveAbilities(abilitySet(15, 14, 22, 8, 10, 10), map[rules.Ability]int{rules.Strength: 2})
	if err != nil || got[rules.Constitution] != 22 || got[rules.Strength] != 17 {
		t.Fatalf("a score already past 20 blocks another = %v %v", got, err)
	}
	for name, inc := range map[string]map[rules.Ability]int{
		"three points": {rules.Strength: 2, rules.Dexterity: 1},
		"one point":    {rules.Strength: 1},
		"past twenty":  {rules.Dexterity: 1, rules.Strength: 1},
		"negative":     {rules.Strength: 3, rules.Dexterity: -1},
		"three ways":   {rules.Strength: 1, rules.Dexterity: 1, rules.Wisdom: 0},
		"unknown":      {"luck": 2},
	} {
		base := abilitySet(15, 20, 13, 8, 10, 10)
		if _, err := rules.ImproveAbilities(base, inc); err == nil {
			t.Errorf("%s accepted", name)
		}
		if base[rules.Strength] != 15 {
			t.Errorf("%s changed the input", name)
		}
	}
}

func TestSpellcastingTables(t *testing.T) {
	t.Parallel()
	cantrips := map[string][3]int{"bard": {2, 3, 4}, "cleric": {3, 4, 5}, "druid": {2, 3, 4}, "sorcerer": {4, 5, 6}, "warlock": {2, 3, 4}, "wizard": {3, 4, 5}, "paladin": {}, "fighter": {}}
	for class, want := range cantrips {
		got := [3]int{rules.CantripsKnown(class, 3), rules.CantripsKnown(class, 4), rules.CantripsKnown(class, 10)}
		if got != want || rules.CantripsKnown(class, 9) != want[1] || rules.CantripsKnown(class, 20) != want[2] {
			t.Errorf("%s cantrips = %v, want %v", class, got, want)
		}
	}
	prepared := map[string][4]int{
		"bard": {4, 5, 16, 22}, "cleric": {4, 5, 16, 22}, "druid": {4, 5, 16, 22}, "sorcerer": {2, 4, 16, 22},
		"wizard": {4, 5, 16, 25}, "warlock": {2, 3, 11, 15}, "paladin": {2, 3, 10, 15}, "ranger": {2, 3, 10, 15}, "rogue": {},
	}
	for class, want := range prepared {
		got := [4]int{rules.PreparedSpells(class, 1), rules.PreparedSpells(class, 2), rules.PreparedSpells(class, 12), rules.PreparedSpells(class, 20)}
		if got != want || rules.PreparedSpells(class, 0) != want[0] || rules.PreparedSpells(class, 25) != want[3] {
			t.Errorf("%s prepared = %v, want %v", class, got, want)
		}
	}
	levels := map[string][6]int{
		"wizard": {1, 1, 2, 3, 5, 9}, "paladin": {1, 1, 1, 2, 3, 5}, "warlock": {1, 1, 2, 3, 5, 5}, "fighter": {},
	}
	for class, want := range levels {
		var got [6]int
		for i, lvl := range []int{1, 2, 3, 5, 9, 20} {
			got[i] = rules.MaxSpellLevel(class, lvl)
		}
		if got != want {
			t.Errorf("%s max spell level = %v, want %v", class, got, want)
		}
	}
}

func TestMulticlassResources(t *testing.T) {
	t.Parallel()
	single := rules.MulticlassResources([]rules.ClassLevel{{Class: "wizard", Level: 3, HitDie: 6}})
	if !reflect.DeepEqual(single, rules.ResourcesAt("wizard", 6, 3)) {
		t.Fatalf("single class = %v", single)
	}
	mixed := rules.MulticlassResources([]rules.ClassLevel{
		{Class: "fighter", Level: 2, HitDie: 10}, {Class: "wizard", Level: 3, HitDie: 6}, {Class: "paladin", Level: 3, HitDie: 10},
	})
	want := []rules.Resource{
		{Key: "hit-dice", Label: "Hit Dice (5d10, 3d6)", Current: 8, Max: 8},
		{Key: "spell-slots-1", Label: "Level 1 spell slots", Current: 4, Max: 4},
		{Key: "spell-slots-2", Label: "Level 2 spell slots", Current: 3, Max: 3},
		{Key: "spell-slots-3", Label: "Level 3 spell slots", Current: 2, Max: 2},
	}
	if !reflect.DeepEqual(mixed, want) {
		t.Fatalf("mixed = %v", mixed)
	}
	pact := rules.MulticlassResources([]rules.ClassLevel{{Class: "warlock", Level: 3, HitDie: 8}, {Class: "sorcerer", Level: 1, HitDie: 6}})
	labels := []string{}
	for _, r := range pact {
		labels = append(labels, r.Key+"="+strings.Repeat("|", r.Max))
	}
	if strings.Join(labels, " ") != "hit-dice=|||| spell-slots-1=|| pact-slots-2=||" {
		t.Fatalf("pact = %v", labels)
	}
	pactOnly := rules.MulticlassResources([]rules.ClassLevel{{Class: "warlock", Level: 2, HitDie: 8}, {Class: "fighter", Level: 1, HitDie: 10}})
	if len(pactOnly) != 2 || pactOnly[1].Key != "spell-slots-1" || pactOnly[1].Label != "Level 1 spell slots" || pactOnly[1].Max != 2 {
		t.Fatalf("pact magic alone = %v", pactOnly)
	}
	martial := rules.MulticlassResources([]rules.ClassLevel{{Class: "fighter", Level: 1, HitDie: 10}, {Class: "rogue", Level: 1, HitDie: 8}})
	if len(martial) != 1 || martial[0].Label != "Hit Dice (1d10, 1d8)" {
		t.Fatalf("martial = %v", martial)
	}
}

func TestRetrainedIncreasesMatchTheImprovementsTaken(t *testing.T) {
	t.Parallel()
	base := abilitySet(15, 14, 13, 8, 10, 10)
	ok := []struct {
		inc  map[rules.Ability]int
		asis int
	}{
		{map[rules.Ability]int{}, 0},
		{map[rules.Ability]int{rules.Strength: 2, rules.Dexterity: 0}, 1},
		{map[rules.Ability]int{rules.Strength: 3, rules.Constitution: 1}, 2},
		{map[rules.Ability]int{rules.Strength: 5, rules.Dexterity: 1}, 3},
	}
	for _, c := range ok {
		if err := rules.CheckIncreases(base, c.inc, c.asis); err != nil {
			t.Errorf("%v with %d: %v", c.inc, c.asis, err)
		}
	}
	bad := []struct {
		inc  map[rules.Ability]int
		asis int
	}{
		{map[rules.Ability]int{rules.Strength: 2}, 0},
		{map[rules.Ability]int{rules.Strength: 1}, 1},
		{map[rules.Ability]int{rules.Strength: 6}, 3},
		{map[rules.Ability]int{"luck": 2}, 1},
		{map[rules.Ability]int{rules.Strength: 3, rules.Dexterity: -1}, 1},
	}
	for _, c := range bad {
		if err := rules.CheckIncreases(base, c.inc, c.asis); err == nil {
			t.Errorf("%v with %d accepted", c.inc, c.asis)
		}
	}
}
