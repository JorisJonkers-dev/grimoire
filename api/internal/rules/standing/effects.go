package standing

// Mode is how the d20 of a check is rolled.
type Mode string

// Modes.
const (
	Straight     Mode = ""
	Advantage    Mode = "advantage"
	Disadvantage Mode = "disadvantage"
)

// Social is what a Standing does to a social check with a member of the Faction.
type Social struct {
	Mode  Mode
	Bonus int
}

// SocialCheck is the effect of a tier on a social check with a member: Advantage when Allied,
// Disadvantage when Hostile, and a small bonus or penalty in between.
func SocialCheck(t Tier) Social {
	switch t {
	case Hostile:
		return Social{Mode: Disadvantage, Bonus: 0}
	case Unfriendly:
		return Social{Mode: Straight, Bonus: -2}
	case Friendly:
		return Social{Mode: Straight, Bonus: 2}
	case Allied:
		return Social{Mode: Advantage, Bonus: 0}
	case Neutral:
	}
	return Social{Mode: Straight, Bonus: 0}
}

// PricePct is what a member Shop adds to its prices at a tier, in percent; less than nothing for a friend.
func PricePct(t Tier) int {
	switch t {
	case Hostile:
		return 50
	case Unfriendly:
		return 20
	case Friendly:
		return -10
	case Allied:
		return -20
	case Neutral:
	}
	return 0
}

// Attitude is how a creature first takes to the party.
type Attitude string

// Attitudes.
const (
	AttitudeHostile     Attitude = "hostile"
	AttitudeIndifferent Attitude = "indifferent"
	AttitudeFriendly    Attitude = "friendly"
)

// FirstReaction is the attitude a member of the Faction starts from at a tier.
func FirstReaction(t Tier) Attitude {
	switch t {
	case Hostile:
		return AttitudeHostile
	case Friendly, Allied:
		return AttitudeFriendly
	case Unfriendly, Neutral:
	}
	return AttitudeIndifferent
}

// Weight is the weight of one of the Faction's own entries on an Encounter Table in its territory: twice
// as likely when Hostile, a fifth as likely when Allied. A weight above nothing never rounds away.
func Weight(t Tier, weight int) int {
	tenths := 10
	switch t {
	case Hostile:
		tenths = 20
	case Unfriendly:
		tenths = 15
	case Friendly:
		tenths = 5
	case Allied:
		tenths = 2
	case Neutral:
	}
	if weight <= 0 {
		return 0
	}
	return max(1, (weight*tenths+5)/10)
}

// maxNameRunes is how much of a Faction's name a Roll Card line carries.
const maxNameRunes = 50

// Line is what a Roll Card says of a Standing with a Faction: the tier and the name, and Advantage or
// Disadvantage when the tier gives one.
func Line(t Tier, faction string) string {
	title := map[Tier]string{Hostile: "Hostile", Unfriendly: "Unfriendly", Neutral: "Neutral", Friendly: "Friendly", Allied: "Allied"}[t]
	if name := []rune(faction); len(name) > maxNameRunes {
		faction = string(name[:maxNameRunes-1]) + "…"
	}
	line := title + " with " + faction
	switch SocialCheck(t).Mode {
	case Advantage:
		return line + ": Advantage"
	case Disadvantage:
		return line + ": Disadvantage"
	case Straight:
	}
	return line
}
