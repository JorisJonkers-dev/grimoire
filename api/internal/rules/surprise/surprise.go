// Package surprise holds the ambush rules: hidden creatures set a Stealth DC, Perception meets it or
// not, and anyone who did not notice the threat rolls initiative at disadvantage.
package surprise

import "slices"

// Passive is a passive score: 10 plus the bonus.
func Passive(bonus int) int {
	return 10 + bonus
}

// DC is what hidden creatures set: the best passive Stealth among them.
func DC(stealth []int) int {
	if len(stealth) == 0 {
		return Passive(0)
	}
	return Passive(slices.Max(stealth))
}

// Notices reports whether a Perception score or roll meets the DC.
func Notices(perception, dc int) bool {
	return perception >= dc
}

// Initiative is the initiative dice: one d20, or the lower of two when surprised.
func Initiative(surprised bool) string {
	if surprised {
		return "2d20kl1"
	}
	return "1d20"
}
