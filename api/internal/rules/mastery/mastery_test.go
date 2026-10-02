package mastery_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/mastery"
)

func TestFindingAWeaponsMastery(t *testing.T) {
	t.Parallel()
	if m, ok := mastery.Of([]string{"Finesse", "Light", "Nick", "Thrown"}); !ok || m != mastery.Nick {
		t.Errorf("a dagger = %s %v", m, ok)
	}
	if m, ok := mastery.Of([]string{"topple", "Versatile"}); !ok || m != mastery.Topple {
		t.Errorf("lower case = %s %v", m, ok)
	}
	if _, ok := mastery.Of([]string{"Heavy", "Two-Handed", "Vexed"}); ok {
		t.Error("no mastery")
	}
	if len(mastery.All()) != 8 {
		t.Error("eight masteries")
	}
}

func TestEachMasteryOnAHitOrAMiss(t *testing.T) {
	t.Parallel()
	none := mastery.Hit{}
	for p, want := range map[mastery.Property]mastery.Hit{
		mastery.Cleave: {Cleave: true}, mastery.Push: {PushFt: 10}, mastery.Sap: {Effect: "sapped"}, mastery.Slow: {Effect: "slowed"},
		mastery.Vex: {Effect: "vexed"}, mastery.Topple: {SaveAbility: "constitution", SaveDC: 14, SaveCondition: "prone"},
		mastery.Graze: none, mastery.Nick: none,
	} {
		if got := mastery.OnHit(p, 6); !reflect.DeepEqual(got, want) {
			t.Errorf("%s on a hit = %+v, want %+v", p, got, want)
		}
	}
	if mastery.OnMiss(mastery.Graze, 3) != 3 || mastery.OnMiss(mastery.Graze, -1) != 0 || mastery.OnMiss(mastery.Sap, 3) != 0 {
		t.Error("only Graze deals damage on a miss, never negative")
	}
	if !mastery.FreeOffHand(mastery.Nick) || mastery.FreeOffHand(mastery.Vex) {
		t.Error("only Nick frees the off-hand attack")
	}
}

func TestMasteredWeapons(t *testing.T) {
	t.Parallel()
	if got := mastery.Mastered([]string{"longsword", "longsword", "shortbow", "dagger"}, 2); !reflect.DeepEqual(got, []string{"longsword", "shortbow"}) {
		t.Errorf("mastered = %v", got)
	}
	if got := mastery.Mastered([]string{"club"}, 0); got != nil {
		t.Errorf("no masteries = %v", got)
	}
}
