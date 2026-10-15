package effects_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// A branch reads as one sentence: branches nested one in another join their tests, and a condition a
// branch puts on reads as what happens to the creature it picked out, since the branch settled the save.
func TestABranchReadsAsOneSentence(t *testing.T) {
	t.Parallel()
	charm := effects.SaveCondition{Ability: "wisdom", Slug: "charmed"}
	stun := effects.SaveCondition{Ability: "constitution", Slug: "stunned"}
	when := func(c effects.Condition, then ...effects.Part) effects.Branch {
		return effects.Branch{When: c, Then: then}
	}
	failed := effects.Condition{Kind: effects.FailsBy, N: 1}
	for _, c := range []struct {
		branch effects.Branch
		want   string
	}{
		{when(failed, charm), "If a creature fails the save, it has the Charmed condition."},
		{
			when(effects.Condition{Kind: effects.CreatureIs, Type: "fey"}, when(failed, charm)),
			"If a creature is a fey and fails the save, it has the Charmed condition.",
		},
		{when(effects.Condition{Kind: effects.FailsBy, N: 5}, stun, charm), "If a creature fails the save by 5 or more, it has the Stunned condition and has the Charmed condition."},
		{when(effects.Condition{Kind: effects.CreatureIs, Type: "undead"}, charm), "If the creature is an undead, it has the Charmed condition."},
		{when(effects.Condition{Kind: effects.FirstEachTurn}, stun), "The first time on a turn the effect touches a creature, it has the Stunned condition."},
		{
			when(effects.Condition{Kind: effects.FirstEachTurn}, when(effects.Condition{Kind: effects.HPAtMost, N: 10}, effects.Dispel{})),
			"If a creature is touched by the effect for the first time on a turn and has 10 Hit Points or fewer, every spell on the target ends.",
		},
		{when(failed, stun, effects.Dispel{}), "If a creature fails the save, on a failed Constitution saving throw, a creature has the Stunned condition. Every spell on the target ends."},
		{when(effects.Condition{Kind: effects.HPAtMost, N: 1}, effects.Dispel{}), "If a creature has 1 Hit Points or fewer, every spell on the target ends."},
	} {
		if got := c.branch.Text(); got != c.want {
			t.Errorf("text =\n%q\nwant\n%q", got, c.want)
		}
	}
}
