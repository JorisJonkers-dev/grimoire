package rules

import (
	"slices"
	"strconv"
	"strings"
)

// SpellbookAllotment is how many spells a wizard's book holds for free at a wizard level: six at first
// level and two more each level after.
func SpellbookAllotment(level int) int {
	return 6 + 2*(max(level, 1)-1)
}

// CopyCost is what copying a spell into a spellbook takes: 50 gold pieces and 2 hours per spell level.
func CopyCost(spellLevel int) (int, int) {
	return 50 * spellLevel, 120 * spellLevel
}

// RitualMinutes is how long casting a spell as a ritual takes: its casting time and 10 minutes more. An
// action, bonus action or reaction counts as no time on the clock.
func RitualMinutes(castingTime string) int {
	return castingMinutes(castingTime) + 10
}

func castingMinutes(text string) int {
	text = strings.TrimSuffix(text, "s")
	for unit, per := range map[string]int{"minute": 1, "hour": 60} {
		if n, ok := strings.CutSuffix(text, unit); ok {
			v, _ := strconv.Atoi(n)
			return v * per
		}
	}
	return 0
}

// CheckPreparation checks a new list of prepared spells: no more than the class prepares, each once, and
// for a class that does not prepare after a long rest, at most one spell swapped out.
func CheckPreparation(class Class, limit int, previous, next []string) error {
	if len(next) > limit {
		return violation("prepare at most %d spells", limit)
	}
	for i, s := range next {
		if slices.Contains(next[:i], s) {
			return violation("prepare each spell once")
		}
	}
	if class.Casting.AfterRest {
		return nil
	}
	dropped := 0
	for _, s := range previous {
		if !slices.Contains(next, s) {
			dropped++
		}
	}
	if dropped > 1 {
		return violation("swap at most one spell when you gain a level")
	}
	return nil
}

// MinutesPerDay is the length of a day on the Game Clock.
const MinutesPerDay = 24 * 60

// AdvanceClock moves the Game Clock on by some minutes, into later days past midnight; it never runs
// backwards.
func AdvanceClock(day, minute, by int) (int, int) {
	total := minute + max(by, 0)
	return day + total/MinutesPerDay, total % MinutesPerDay
}
