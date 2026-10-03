// Package mounted holds the rules of riding: what mounting costs, what a controlled mount may do, and
// when a rider falls off.
package mounted

import "github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"

// FallDC is the Dexterity save a rider makes to keep its seat.
const FallDC = 10

// feetPerStep is the grain movement is counted in.
const feetPerStep = 5

// Cost is the movement mounting or dismounting takes: half the rider's Speed, counted in whole steps of
// 5 feet and never less than one step for a creature that can move.
func Cost(speedFt int) int {
	if speedFt <= 0 {
		return 0
	}
	return max(feetPerStep, speedFt/2/feetPerStep*feetPerStep)
}

// Falls reports whether a rider's save fails to keep it in the saddle.
func Falls(total int) bool {
	return total < FallDC
}

// Allows reports whether a mount may take an action: a controlled mount only Dashes, Disengages or
// Dodges, and an independent one acts as it likes.
func Allows(controlled bool, a actions.Action) bool {
	return !controlled || a == actions.Dash || a == actions.Disengage || a == actions.Dodge
}
