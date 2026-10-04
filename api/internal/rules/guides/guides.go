// Package guides works the compendium's guides out of its own entries: which magic items suit which
// levels of play, and what the attacks of monsters of a Challenge Rating look like.
package guides

import (
	"slices"
	"strconv"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// Tier is a band of character levels and the magic item rarities that suit a party in it.
type Tier struct {
	No       int
	From     int
	To       int
	Rarities []string
}

// rarities are the rarities the loot guide offers, in the order the tiers open them.
func rarities() []string {
	return []string{"common", "uncommon", "rare", "very-rare", "legendary"}
}

// Tiers are the four tiers of play. Each offers every rarity the one before it does and one more; an
// artifact is never offered.
func Tiers() []Tier {
	all := rarities()
	return []Tier{
		{No: 1, From: 1, To: 4, Rarities: slices.Clone(all[:2])},
		{No: 2, From: 5, To: 10, Rarities: slices.Clone(all[:3])},
		{No: 3, From: 11, To: 16, Rarities: slices.Clone(all[:4])},
		{No: 4, From: 17, To: 20, Rarities: slices.Clone(all)},
	}
}

// FirstTier is the first tier of play a rarity suits, or 0 for one the guide never offers.
func FirstTier(rarity string) int {
	for _, t := range Tiers() {
		if slices.Contains(t.Rarities, rarity) {
			return t.No
		}
	}
	return 0
}

// Challenge writes a Challenge Rating as the books do: 1/8, 1/4, 1/2 or a number.
func Challenge(cr float64) string {
	if cr == 0.125 {
		return "1/8"
	}
	if cr == 0.25 {
		return "1/4"
	}
	if cr == 0.5 {
		return "1/2"
	}
	return strconv.FormatFloat(cr, 'f', -1, 64)
}

// Attack is one attack of a monster: its bonus to hit, and the dice and flat bonus of its damage.
type Attack struct {
	ToHit     int
	Dice      string
	Bonus     int
	ExtraDice string
}

// rolled is what dice come to on average; dice that cannot be read come to nothing.
func rolled(notation string) float64 {
	spec, err := dice.Parse(notation)
	if err != nil {
		return 0
	}
	sum := 0.0
	for _, g := range spec.Groups {
		sum += float64(g.Count) * float64(g.Faces+1) / 2
	}
	return sum
}

// Average is the damage a hit deals on average: its dice, its bonus and any extra dice, never less than none.
func Average(a Attack) float64 {
	return max(0, rolled(a.Dice)+float64(a.Bonus)+rolled(a.ExtraDice))
}

// Band sums up the attacks of the monsters of one Challenge Rating: how many there are, the lowest,
// middle and highest bonus to hit, and the middle damage of a hit, rounded down.
type Band struct {
	Attacks   int
	ToHitLow  int
	ToHit     int
	ToHitHigh int
	Damage    int
}

// Summarise sums attacks up. With an even number of them the middle is the lower of the two in the middle.
func Summarise(attacks []Attack) Band {
	if len(attacks) == 0 {
		return Band{Attacks: 0, ToHitLow: 0, ToHit: 0, ToHitHigh: 0, Damage: 0}
	}
	hits, damage := make([]int, 0, len(attacks)), make([]float64, 0, len(attacks))
	for _, a := range attacks {
		hits, damage = append(hits, a.ToHit), append(damage, Average(a))
	}
	slices.Sort(hits)
	slices.Sort(damage)
	mid := (len(attacks) - 1) / 2
	return Band{Attacks: len(attacks), ToHitLow: hits[0], ToHit: hits[mid], ToHitHigh: hits[len(hits)-1], Damage: int(damage[mid])}
}
