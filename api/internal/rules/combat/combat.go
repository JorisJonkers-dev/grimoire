// Package combat holds the turn rules: initiative ranks and the action economy.
package combat

import "sort"

// Resource is a part of the action economy that a turn spends.
type Resource int

// Resources.
const (
	Action Resource = iota
	BonusAction
	Reaction
)

// Economy is what a Combatant has left: true means still available.
type Economy struct {
	Action      bool
	BonusAction bool
	Reaction    bool
	MovementFt  int
}

// Fresh is the economy at the start of a turn.
func Fresh(speedFt int) Economy {
	return Economy{Action: true, BonusAction: true, Reaction: true, MovementFt: speedFt}
}

// Spend uses one resource; ok is false when it is already spent.
func (e Economy) Spend(r Resource) (Economy, bool) {
	switch r {
	case Action:
		ok := e.Action
		e.Action = false
		return e, ok
	case BonusAction:
		ok := e.BonusAction
		e.BonusAction = false
		return e, ok
	case Reaction:
		ok := e.Reaction
		e.Reaction = false
		return e, ok
	}
	return e, false
}

// Move spends movement; ok is false when the move is longer than what is left.
func (e Economy) Move(costFt int) (Economy, bool) {
	if costFt < 0 || costFt > e.MovementFt {
		return e, false
	}
	e.MovementFt -= costFt
	return e, true
}

// Counts lists the distinct initiative totals, highest first. Equal totals share a count and act
// simultaneously, in any order.
func Counts(totals []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, t := range totals {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

// Rank is the 1-based place of a total among the counts; tied totals share a rank.
func Rank(totals []int, total int) int {
	rank := 1
	for _, c := range Counts(totals) {
		if c > total {
			rank++
		}
	}
	return rank
}

// Next is the count that acts after current, and whether a new round begins with it. With no totals
// there is nothing to act and next is current.
func Next(totals []int, current int) (next int, newRound bool) {
	counts := Counts(totals)
	for _, c := range counts {
		if c < current {
			return c, false
		}
	}
	if len(counts) == 0 {
		return current, false
	}
	return counts[0], true
}
