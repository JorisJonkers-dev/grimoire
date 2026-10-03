package itembuild

import (
	"strconv"
	"strings"
)

// PriceCheck compares an item's rarity and value with what its properties are worth.
type PriceCheck struct {
	// Points weigh the item's magic; Suggested is the rarity they point to.
	Points    int
	Suggested string
	// PriceGP is the guide price of the rarity the author chose; a value within half and double of it fits.
	PriceGP int
	Fits    bool
	Notes   []string
}

// guidePrices are the 2024 guide prices by rarity, cheapest first.
func guidePrices() map[string]int {
	return map[string]int{"common": 100, "uncommon": 400, "rare": 4000, "very_rare": 40000, "legendary": 200000}
}

// Points weighs what a design's magic is worth: its enchantment, each Item Property, and its charges;
// a curse takes some back, and a consumable is worth half.
func Points(d Design) int {
	points := 2*d.Enchantment + d.chargePoints()
	consumable := false
	for _, p := range d.Properties {
		points += p.points()
		consumable = consumable || p.Type == Consumable
	}
	if consumable {
		points /= 2
	}
	return max(0, points)
}

func (d Design) chargePoints() int {
	if d.Charges == nil {
		return 0
	}
	return (d.Charges.Max + 2) / 3
}

func (p Property) points() int {
	switch p.Type {
	case SkillBoost:
		return map[string]int{"advantage": 1, "d4": 1, "flat": p.Value, "proficiency": 1, "expertise": 2}[p.Mode]
	case Bonus:
		return 2 * p.Value
	case Resistance, Growth:
		return 2
	case ExtraDamage:
		return 1 + diceAverage(p.Dice)/3
	case SenseRow, Cantrip, Sentient, SetBonus, Container:
		return 1
	case SpeedRow:
		if p.Speed == "fly" {
			return 3
		}
		return 1
	case SpellRow:
		return 1 + p.Level
	case Curse:
		return -2
	}
	return 0
}

// diceAverage is a die expression's average, rounded down: 2d6 is 7.
func diceAverage(dice string) int {
	n, faces, _ := strings.Cut(dice, "d")
	count, _ := strconv.Atoi(n)
	size, _ := strconv.Atoi(faces)
	return count * (size + 1) / 2
}

// Suggest is the rarity a weight of magic points to.
func Suggest(points int) string {
	switch {
	case points <= 1:
		return "common"
	case points <= 3:
		return "uncommon"
	case points <= 6:
		return "rare"
	case points <= 9:
		return "very_rare"
	}
	return "legendary"
}

// Price prices a design: the rarity its magic points to, the guide price of its own rarity, and whether
// both fit.
func Price(d Design) PriceCheck {
	points := Points(d)
	c := PriceCheck{Points: points, Suggested: Suggest(points), PriceGP: guidePrices()[d.Rarity], Fits: true, Notes: nil}
	if c.Suggested != d.Rarity {
		c.Fits = false
		c.Notes = append(c.Notes, "Its properties point to "+words(c.Suggested)+", not "+words(d.Rarity)+".")
	}
	if d.ValueGP*2 < c.PriceGP || d.ValueGP > 2*c.PriceGP {
		c.Fits = false
		c.Notes = append(c.Notes, "A "+words(d.Rarity)+" item goes for about "+strconv.Itoa(c.PriceGP)+" gp; "+strconv.Itoa(d.ValueGP)+" gp is far from it.")
	}
	return c
}

func words(s string) string { return strings.ReplaceAll(s, "_", " ") }
