package effects_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// An initial save is checked before it is stored: an ability the rules know, and a DC from 1 to 30 or
// none, which is the caster's spell save DC.
func TestTheInitialSaveIsChecked(t *testing.T) {
	t.Parallel()
	for _, ok := range []effects.InitialSave{{}, {Ability: "wisdom"}, {Ability: "strength", DC: 30}, {Ability: "charisma", DC: 1}} {
		if err := ok.Check(); err != nil {
			t.Errorf("%+v: %v", ok, err)
		}
	}
	for _, c := range []struct {
		bad  effects.InitialSave
		want string
	}{
		{effects.InitialSave{Ability: "luck"}, "ability: choose one of the options"},
		{effects.InitialSave{Ability: "wisdom", DC: 31}, "dc: runs from 1 to 30"},
		{effects.InitialSave{Ability: "wisdom", DC: -1}, "dc: runs from 1 to 30"},
		{effects.InitialSave{DC: 12}, "ability: name the ability"},
	} {
		if err := c.bad.Check(); err == nil || err.Error() != c.want {
			t.Errorf("%+v: %v, want %q", c.bad, err, c.want)
		}
	}
}

// The initial save reads as the spell prints it, after its range and targets.
func TestTheInitialSaveReadsAsRulesText(t *testing.T) {
	t.Parallel()
	d := effects.Definition{
		Targeting: effects.Targeting{Kind: effects.TargetCreatures, RangeFt: 60},
		Save:      effects.InitialSave{Ability: "wisdom"},
		Parts:     []effects.Part{effects.Immobile{}},
	}
	want := []string{
		"Range: 60 feet. Targets: a creature.",
		"Each target makes a Wisdom saving throw against your spell save DC; the effect lands on a failure.",
		effects.Immobile{}.Text(),
	}
	if got := d.Text(); !reflect.DeepEqual(got, want) {
		t.Fatalf("text = %q", got)
	}
	if got := (effects.InitialSave{Ability: "dexterity", DC: 15}).Text(); got != "Each target makes a DC 15 Dexterity saving throw; the effect lands on a failure." {
		t.Fatalf("fixed = %q", got)
	}
	if got := (effects.InitialSave{}).Text(); got != "" {
		t.Fatalf("none = %q", got)
	}
}
