// Package mastery describes the 2024 Weapon Mastery properties: what each does when an attack with a
// mastered weapon hits or misses.
package mastery

import (
	"slices"
	"strings"
)

// Property is a weapon's mastery.
type Property string

// The Weapon Mastery properties.
const (
	Cleave Property = "cleave"
	Graze  Property = "graze"
	Nick   Property = "nick"
	Push   Property = "push"
	Sap    Property = "sap"
	Slow   Property = "slow"
	Topple Property = "topple"
	Vex    Property = "vex"
)

// All lists every mastery, in the order the rules list them.
func All() []Property {
	return []Property{Cleave, Graze, Nick, Push, Sap, Slow, Topple, Vex}
}

// Of finds the mastery among a weapon's properties, by name in any case.
func Of(properties []string) (Property, bool) {
	for _, p := range properties {
		for _, m := range All() {
			if strings.EqualFold(p, string(m)) {
				return m, true
			}
		}
	}
	return "", false
}

// Hit is what a mastery does once an attack hits: an Effect it puts on the target until the attacker's
// next turn, a push, a saving throw against a condition, or a second attack.
type Hit struct {
	Effect        string
	PushFt        int
	SaveAbility   string
	SaveDC        int
	SaveCondition string
	Cleave        bool
}

// OnHit is what a mastery does when an attack with toHit hits. Topple's DC is 8 plus the attack's
// ability modifier and proficiency bonus, which is the attack bonus itself.
func OnHit(p Property, toHit int) Hit {
	h := Hit{Effect: "", PushFt: 0, SaveAbility: "", SaveDC: 0, SaveCondition: "", Cleave: false}
	switch p {
	case Cleave:
		h.Cleave = true
	case Push:
		h.PushFt = 10
	case Sap:
		h.Effect = "sapped"
	case Slow:
		h.Effect = "slowed"
	case Vex:
		h.Effect = "vexed"
	case Topple:
		h.SaveAbility, h.SaveDC, h.SaveCondition = "constitution", 8+toHit, "prone"
	case Graze, Nick:
	}
	return h
}

// OnMiss is the damage a mastery deals when an attack misses: Graze deals the ability modifier.
func OnMiss(p Property, damageMod int) int {
	if p == Graze {
		return max(0, damageMod)
	}
	return 0
}

// FreeOffHand reports whether the off-hand attack is part of the Attack action instead of a Bonus Action.
func FreeOffHand(p Property) bool {
	return p == Nick
}

// Mastered picks the weapons a creature masters: the ones it carries, in order, up to its count.
func Mastered(carried []string, count int) []string {
	var out []string
	for _, w := range carried {
		if len(out) < count && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}
