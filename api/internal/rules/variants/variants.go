// Package variants holds the built-in Rule Variants a DM switches on for a Campaign, and the rule each
// one changes.
package variants

import (
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// The built-in Rule Variants.
const (
	Flanking           = "flanking"
	CriticalFumble     = "critical-fumble"
	CriticalHits       = "critical-hits"
	Rests              = "rests"
	ShortRestCap       = "short-rest-cap"
	MassiveDamage      = "massive-damage"
	HiddenDeathSaves   = "hidden-death-saves"
	HealingSurge       = "healing-surge"
	SlowNaturalHealing = "slow-natural-healing"
	Morale             = "morale"
	Encumbrance        = "encumbrance"
)

// What a variant can be.
const (
	Off                 = "off"
	On                  = "on"
	CritDoubleDice      = "double-dice"
	CritMaxDice         = "max-dice"
	RestsStandard       = "standard"
	RestsGritty         = "gritty"
	RestsEpic           = "epic"
	NoCap               = "none"
	EncumbranceStandard = "standard"
	EncumbranceVariant  = "variant"
)

const (
	// SystemShockDC is the Constitution save against massive damage.
	SystemShockDC = 15
	// MoraleDC is the Wisdom save a creature makes to stand its ground.
	MoraleDC = 10
)

// Option is one thing a Rule Variant can be.
type Option struct {
	Value string
	Label string
}

// Variant is a built-in Rule Variant. Its first option is how the rules play without it. Automated
// says Grimoire applies it in play; the others are the DM's to apply by hand.
type Variant struct {
	Slug        string
	Name        string
	Description string
	Options     []Option
	Automated   bool
}

func onOff() []Option {
	return []Option{{Off, "Off"}, {On, "On"}}
}

// Catalogue lists the built-in Rule Variants.
func Catalogue() []Variant {
	return []Variant{
		{Flanking, "Flanking", "A melee attack has Advantage when an ally who can act stands on the hex straight across the target.", onOff(), true},
		{CriticalFumble, "Critical fumbles", "A natural 1 on an attack roll is a fumble: something goes wrong for the attacker, as the DM rules.", onOff(), false},
		{CriticalHits, "Critical hits", "What a Critical Hit does to the damage dice.", []Option{
			{CritDoubleDice, "Roll every damage die twice"}, {CritMaxDice, "Roll the dice once and add the most they could show"},
		}, true},
		{Rests, "Rest lengths", "How long a rest takes on the Game Clock.", []Option{
			{RestsStandard, "Standard: a Short Rest is 1 hour, a Long Rest 8 hours"},
			{RestsGritty, "Gritty: a Short Rest is 8 hours, a Long Rest 7 days"},
			{RestsEpic, "Epic: a Short Rest is 5 minutes, a Long Rest 1 hour"},
		}, true},
		{ShortRestCap, "Short Rest cap", "How many Short Rests the party may take between two Long Rests.", []Option{
			{NoCap, "No cap"}, {"1", "One"}, {"2", "Two"}, {"3", "Three"},
		}, true},
		{MassiveDamage, "Massive damage", "A creature that takes half its hit point maximum or more from one blow makes a DC 15 Constitution save or suffers system shock.", onOff(), false},
		{HiddenDeathSaves, "Hidden death saves", "The DM rolls death saving throws out of sight, and only the DM knows how they stand.", onOff(), false},
		{HealingSurge, "Healing surges", "As an action, a Character spends up to half its Hit Dice to heal.", onOff(), false},
		{SlowNaturalHealing, "Slow natural healing", "A Long Rest gives back no hit points; a Character spends Hit Dice during it instead.", onOff(), true},
		{Morale, "Morale", "A creature down to half its hit points, or whose side has lost half its number, makes a DC 10 Wisdom save or flees.", onOff(), false},
		{Encumbrance, "Encumbrance", "How a heavy load slows a creature.", []Option{
			{EncumbranceStandard, "Standard: past 15 lb a point of Strength, 5 feet"},
			{EncumbranceVariant, "Variant: 10 feet slower past 5 lb a point, 20 feet past 10 lb"},
			{Off, "Off: a load never slows anyone"},
		}, false},
	}
}

func find(slug string) (found Variant, known bool) {
	for _, v := range Catalogue() {
		if v.Slug == slug {
			return v, true
		}
	}
	return found, false
}

// Valid reports whether a Rule Variant exists and can be that.
func Valid(slug, value string) bool {
	v, known := find(slug)
	return known && slices.ContainsFunc(v.Options, func(o Option) bool { return o.Value == value })
}

// Set is what a Campaign has each Rule Variant at.
type Set map[string]string

// Get is what a variant is at for the Campaign: what was chosen, or how the rules play without it.
// An unknown variant is at nothing.
func (s Set) Get(slug string) string {
	v, known := find(slug)
	if !known {
		return ""
	}
	if value := s[slug]; Valid(slug, value) {
		return value
	}
	return v.Options[0].Value
}

// On reports whether a variant that is on or off is on.
func (s Set) On(slug string) bool {
	return s.Get(slug) == On
}

// Flanks reports whether an attacker next to its target has it flanked: an ally stands on the hex
// straight across the target.
func Flanks(attacker, ally, target hex.Coord) bool {
	across := hex.Coord{Q: 2*target.Q - attacker.Q, R: 2*target.R - attacker.R}
	return hex.Distance(attacker, target) == 1 && ally == across
}

// Fumbles reports whether an attack's d20 is a critical fumble.
func Fumbles(natural int) bool {
	return natural == 1
}

// CriticalDice is the damage a Critical Hit rolls: every die twice, or with max-dice every die once
// plus the most the dice could show, which comes back as a flat amount.
func CriticalDice(mode string, s dice.Spec) (dice.Spec, int) {
	if mode != CritMaxDice {
		return attack.CriticalDice(s), 0
	}
	extra := 0
	for _, g := range s.Groups {
		if g.Sign == 1 {
			extra += g.Count * g.Faces
		}
	}
	return s, extra
}

// RestMinutes is how long a Short or Long Rest takes on the Game Clock.
func RestMinutes(mode string, long bool) int {
	short, full := 60, 8*60
	switch mode {
	case RestsGritty:
		short, full = 8*60, 7*24*60
	case RestsEpic:
		short, full = 5, 60
	}
	if long {
		return full
	}
	return short
}

// ShortRestAllowed reports whether the cap leaves room for another Short Rest, given how many the
// party has taken since its last Long Rest.
func ShortRestAllowed(limit string, taken int) bool {
	for n, value := range []string{"1", "2", "3"} {
		if limit == value {
			return taken < n+1
		}
	}
	return true
}

// SystemShock reports whether one blow is massive damage: half the creature's hit point maximum or more.
func SystemShock(damage, hpMax int) bool {
	return hpMax > 0 && damage*2 >= hpMax
}

// ShowsDeathSaves reports whether a screen sees how a dying Character's death saves stand.
func (s Set) ShowsDeathSaves(dm bool) bool {
	return dm || !s.On(HiddenDeathSaves)
}

// SurgeDice is how many Hit Dice a healing surge may spend: half the Character's level, at least one,
// and no more than it has left.
func SurgeDice(level, left int) int {
	return max(0, min(max(1, level/2), left))
}

// LongRestHP is a Character's hit points after a Long Rest.
func (s Set) LongRestHP(hp, hpMax int) int {
	if s.On(SlowNaturalHealing) {
		return hp
	}
	return hpMax
}

// MoraleCheck reports whether a creature must check its morale: it is down to half its hit points or
// fewer, or half its side has fallen.
func MoraleCheck(hp, hpMax, fallen, side int) bool {
	return (hpMax > 0 && hp*2 <= hpMax) || (fallen > 0 && fallen*2 >= side)
}

// BurdenedSpeed is a creature's speed under what it carries. Past 15 pounds a point of Strength it
// drags its gear at 5 feet, and past twice that it cannot move. The variant also takes 10 feet off
// past 5 pounds a point and 20 past 10.
func BurdenedSpeed(mode string, speed int, weight float64, strength int) int {
	light, heavy, most := float64(strength*5), float64(strength*10), float64(strength*15)
	if mode == Off {
		return speed
	}
	if weight > 2*most {
		return 0
	}
	if weight > most {
		return min(speed, 5)
	}
	if mode != EncumbranceVariant {
		return speed
	}
	if weight > heavy {
		return max(speed-20, 0)
	}
	if weight > light {
		return max(speed-10, 0)
	}
	return speed
}
