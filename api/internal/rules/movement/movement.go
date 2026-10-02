// Package movement holds the physical rules beyond walking: falling, jumping and throwing.
package movement

import "strconv"

// FallMaxDice is the most d6s a fall deals.
const FallMaxDice = 20

// Fall is the bludgeoning damage a fall of so many feet deals, 1d6 per full 10 feet up to 20d6; a
// creature that takes any lands Prone.
func Fall(feet int) (dice string, prone bool) {
	n := min(FallMaxDice, max(0, feet)/10)
	if n == 0 {
		return "", false
	}
	return strconv.Itoa(n) + "d6", true
}

// LongJumpFt is how far a creature leaps: its Strength score in feet after a 10-foot run, half that
// from a standing start.
func LongJumpFt(strength int, running bool) int {
	if running {
		return max(0, strength)
	}
	return max(0, strength) / 2
}

// HighJumpFt is how high a creature leaps: 3 plus its Strength modifier in feet after a run, half that
// standing, never less than 0.
func HighJumpFt(strengthMod int, running bool) int {
	ft := max(0, 3+strengthMod)
	if running {
		return ft
	}
	return ft / 2
}

// Running reports whether a creature has moved far enough this turn to jump with a run-up.
func Running(movedFt int) bool {
	return movedFt >= 10
}

// ThrowRangeFt is how far a creature throws a creature it holds or a small object: 5 feet for each
// point of Strength modifier, at least 5.
func ThrowRangeFt(strengthMod int) int {
	return 5 * max(1, strengthMod)
}

// ThrownDice is the bludgeoning damage a thrown creature or object deals where it lands.
const ThrownDice = "1d6"
