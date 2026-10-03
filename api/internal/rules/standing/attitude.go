package standing

// baseInfluenceDC is what swaying a creature is rolled against, unless its Intelligence is higher.
const baseInfluenceDC = 15

// swayMargin is how far a check must miss by to sour the creature it was meant to sway.
const swayMargin = 5

// InfluenceDC is the DC of an Influence check against a creature of that Intelligence.
func InfluenceDC(intelligence int) int {
	return max(baseInfluenceDC, intelligence)
}

// AttitudeMode is what a creature's attitude does to a check to sway it: Advantage when Friendly,
// Disadvantage when Hostile.
func AttitudeMode(a Attitude) Mode {
	switch a {
	case AttitudeFriendly:
		return Advantage
	case AttitudeHostile:
		return Disadvantage
	case AttitudeIndifferent:
	}
	return Straight
}

// Sway is a creature's attitude after an Influence check: a step towards Friendly when the check
// meets the DC, a step towards Hostile when it misses by five or more, else as it was.
func Sway(a Attitude, total, dc int) Attitude {
	steps := []Attitude{AttitudeHostile, AttitudeIndifferent, AttitudeFriendly}
	at := 1
	for i, s := range steps {
		if s == a {
			at = i
		}
	}
	switch {
	case total >= dc:
		at = min(at+1, len(steps)-1)
	case total <= dc-swayMargin:
		at = max(at-1, 0)
	}
	return steps[at]
}
