// Package tactics proposes a creature's Suggested Action from what it has observed, never from hidden
// numbers, and never coordinated with other creatures.
package tactics

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

// Style is how a creature picks its Suggested Action.
type Style int

// Styles.
const (
	Off Style = iota
	Simple
	Cunning
)

// Overrides the DM can set per creature; anything else follows Intelligence.
const (
	FromIntelligence = "auto"
	OverrideSimple   = "simple"
	OverrideCunning  = "cunning"
	OverrideOff      = "off"
)

// For is the style a creature uses: the DM's override, or Cunning from Intelligence 12 up.
func For(override string, intelligence int) Style {
	switch override {
	case OverrideSimple:
		return Simple
	case OverrideCunning:
		return Cunning
	case OverrideOff:
		return Off
	}
	if intelligence >= 12 {
		return Cunning
	}
	return Simple
}

// Attack is how far one of the creature's attacks reaches.
type Attack struct {
	ReachFt     int
	RangeFt     int
	LongRangeFt int
}

// Target is an enemy the creature can see, with the ranged damage it has seen that enemy deal. March is
// its place in the Marching Order, from 1 at the front; 0 for one that has none.
type Target struct {
	ID               string
	DistanceFt       int
	RangedDamageSeen int
	March            int
}

// rank orders targets by the Marching Order: the front first, and those with no place last.
func (t Target) rank() int {
	if t.March <= 0 {
		return math.MaxInt
	}
	return t.March
}

// Reason says why a suggestion was made.
type Reason int

// Reasons.
const (
	Nearest Reason = iota
	Sniper
	Approach
)

// Suggestion is an attack and its target; with Approach there is no attack in reach yet and Attack is -1.
type Suggestion struct {
	Attack int
	Target Target
	Reason Reason
}

// Suggest picks the creature's action. Simple attacks the nearest enemy, and of several equally near
// the one furthest to the front of the Marching Order. Cunning, if it has a ranged
// attack, shoots the enemy it has seen deal the most damage from range; otherwise it too goes nearest.
func Suggest(s Style, attacks []Attack, targets []Target) (Suggestion, bool) {
	if s == Off || len(targets) == 0 || len(attacks) == 0 {
		return Suggestion{Attack: -1, Target: Target{ID: "", DistanceFt: 0, RangedDamageSeen: 0, March: 0}, Reason: Nearest}, false
	}
	ordered := slices.Clone(targets)
	slices.SortFunc(ordered, func(a, b Target) int {
		return cmp.Or(cmp.Compare(a.DistanceFt, b.DistanceFt), cmp.Compare(a.rank(), b.rank()), strings.Compare(a.ID, b.ID))
	})
	if s == Cunning {
		if sniped, ok := snipe(attacks, ordered); ok {
			return sniped, true
		}
	}
	near := ordered[0]
	if i := choose(attacks, near.DistanceFt, false); i >= 0 {
		return Suggestion{Attack: i, Target: near, Reason: Nearest}, true
	}
	return Suggestion{Attack: -1, Target: near, Reason: Approach}, true
}

// snipe finds the enemy seen dealing the most ranged damage that a ranged attack still reaches.
func snipe(attacks []Attack, ordered []Target) (Suggestion, bool) {
	var best Suggestion
	found := false
	for _, t := range ordered {
		i := choose(attacks, t.DistanceFt, true)
		if i >= 0 && t.RangedDamageSeen > 0 && (!found || t.RangedDamageSeen > best.Target.RangedDamageSeen) {
			best, found = Suggestion{Attack: i, Target: t, Reason: Sniper}, true
		}
	}
	return best, found
}

// choose picks the attack to use at a distance: one in reach, else one in normal range, else one in long
// range. With rangedOnly, reach does not count.
func choose(attacks []Attack, distFt int, rangedOnly bool) int {
	for _, fits := range []func(Attack) bool{
		func(a Attack) bool { return !rangedOnly && distFt <= a.ReachFt },
		func(a Attack) bool { return a.RangeFt > 0 && distFt <= a.RangeFt },
		func(a Attack) bool { return a.RangeFt > 0 && distFt <= a.LongRangeFt },
	} {
		for i, a := range attacks {
			if fits(a) {
				return i
			}
		}
	}
	return -1
}
