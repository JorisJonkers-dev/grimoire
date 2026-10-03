package rules

import (
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// HitPointsAt is a new Character's hit point maximum at a starting level: the whole Hit Die at first
// level, then the fixed average of each later one, each level at least 1.
func HitPointsAt(hitDie, conMod, level int) int {
	return FirstLevelHP(hitDie, conMod) + max(level-1, 0)*max(1, hitDie/2+1+conMod)
}

// RollAbilityScores rolls six scores of 4d6, each dropping its lowest die.
func RollAbilityScores(src dice.Source) []int {
	out := make([]int, 0, 6)
	for range 6 {
		rolls := []int{dice.Face(src, 6), dice.Face(src, 6), dice.Face(src, 6), dice.Face(src, 6)}
		slices.Sort(rolls)
		out = append(out, rolls[1]+rolls[2]+rolls[3])
	}
	return out
}

// ValidateRolled checks rolled base scores are the six the server rolled, placed in any order.
func ValidateRolled(base map[Ability]int, rolled []int) error {
	got := make([]int, 0, len(base))
	for _, s := range base {
		got = append(got, s)
	}
	want := slices.Clone(rolled)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return violation("place the six scores you rolled")
	}
	return nil
}
