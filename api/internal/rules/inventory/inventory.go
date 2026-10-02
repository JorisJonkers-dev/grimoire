// Package inventory holds the rules for carrying gear: which equipment slot takes what, how much a
// creature carries, and what drinking a healing potion restores.
package inventory

import "strings"

// Equipment slots.
const (
	Head       = "head"
	Cloak      = "cloak"
	Body       = "armor"
	Hands      = "hands"
	Feet       = "feet"
	Amulet     = "neck"
	Ring1      = "ring_1"
	Ring2      = "ring_2"
	MainHand   = "main_hand"
	OffHand    = "off_hand"
	Ranged     = "ranged_main"
	Ammunition = "ammunition"
	Instrument = "instrument"
)

// Slots lists the equipment slots around a Character's figure, head to toe, then hands and gear.
func Slots() []string {
	return []string{Head, Cloak, Amulet, Body, Hands, Ring1, Ring2, Feet, MainHand, OffHand, Ranged, Ammunition, Instrument}
}

// Fits reports whether an item of a category goes in a slot. Worn magic items go where they are worn;
// a shield goes in the off hand.
func Fits(slot, category, slug string) bool {
	switch slot {
	case MainHand, Ranged:
		return category == "weapon"
	case OffHand:
		return category == "weapon" || slug == "shield"
	case Body:
		return category == "armor" && slug != "shield"
	case Head, Cloak, Hands, Feet, Amulet:
		return category == "wondrous-item"
	case Ring1, Ring2:
		return category == "ring"
	case Ammunition:
		return category == "ammunition"
	case Instrument:
		return strings.HasPrefix(slug, "musical-instrument")
	}
	return false
}

// Capacity is how many pounds a Small or Medium creature carries: 15 per point of Strength.
func Capacity(strength int) float64 {
	return float64(strength * 15)
}

// Load is how a creature's burden slows it.
type Load string

// Loads.
const (
	Unburdened Load = "none"
	Encumbered Load = "encumbered"
	Immobile   Load = "immobile"
)

// LoadOf is the load of a weight against a capacity: past it a creature drags its gear at 5 feet; past
// twice it, it cannot move.
func LoadOf(weight, capacity float64) Load {
	switch {
	case weight <= capacity:
		return Unburdened
	case weight <= 2*capacity:
		return Encumbered
	default:
		return Immobile
	}
}

// Speed is a creature's speed under a load.
func Speed(speed int, load Load) int {
	switch load {
	case Encumbered:
		return min(speed, 5)
	case Immobile:
		return 0
	case Unburdened:
	}
	return speed
}

// CoinWeight is what a purse weighs: fifty coins to the pound.
func CoinWeight(coins map[string]int) float64 {
	n := 0
	for _, c := range coins {
		n += c
	}
	return float64(n) / 50
}

// Potion is the hit points a healing potion restores: a number of d4s and a flat bonus.
type Potion struct {
	Dice  int
	Faces int
	Bonus int
}

// Healing is what a healing potion restores; false for any other potion.
func Healing(slug string) (Potion, bool) {
	p, ok := map[string]Potion{
		"potion-of-healing": {2, 4, 2}, "potion-of-greater-healing": {4, 4, 4}, "potion-of-superior-healing": {8, 4, 8}, "potion-of-supreme-healing": {10, 4, 20},
	}[slug]
	return p, ok
}
