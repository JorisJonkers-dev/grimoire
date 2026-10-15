package conditionbuild_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// A design saved as rows of the builder's own part types, every type among them, checks and builds the
// same Effect.
func TestDesignsSavedAsRowsStillBuild(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("m", 500)
	raw := `{"icon":"snow","color":"#7fa8dd","text":"Cold seeps in.","ends":"rest","stacks":true,"maxLevel":4,
"perLevel":{"d20":1,"speedFt":5,"deathAt":0},"parts":[{"type":"attack_disadvantage"},{"type":"attacked_advantage"},
{"type":"save_disadvantage","ability":"dexterity"},{"type":"save_fails","ability":"strength"},{"type":"save_advantage","ability":"constitution"},
{"type":"attack_advantage"},{"type":"attacked_disadvantage"},{"type":"incapacitated"},{"type":"immobile"},{"type":"speed_penalty","feet":10},
{"type":"crit_within","feet":5},{"type":"manual","text":"` + long + `"}]}`
	var d conditionbuild.Design
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	if err := conditionbuild.Check(d); err != nil {
		t.Fatal(err)
	}
	edge := func(against, advantage bool) effects.Part {
		return effects.Edge{Against: against, Advantage: advantage, Range: effects.AnyRange, SourceOnly: false}
	}
	want := []effects.Part{
		effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 0, MaxLevel: 4},
		edge(false, false), edge(true, true),
		effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveDisadvantage},
		effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
		effects.SaveEdge{Ability: "constitution", Mode: effects.SaveAdvantage},
		edge(false, true), edge(true, false),
		effects.Incapacitated{},
		effects.Immobile{},
		effects.SpeedPenalty{Ft: 10},
		effects.CritWithin{Feet: 5},
		effects.Manual{Instruction: long},
	}
	if got := conditionbuild.Compile("hb-frost", "Frostbite", d).Definition.Parts; !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %+v", got)
	}
}
