// Package attack resolves attack rolls: hit chance, outcome, range bands and damage.
package attack

import (
	"math"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// Mode is how the d20 is rolled.
type Mode int

// Modes.
const (
	Normal Mode = iota
	Advantage
	Disadvantage
)

// String names the mode as the API does.
func (m Mode) String() string {
	switch m {
	case Advantage:
		return "advantage"
	case Disadvantage:
		return "disadvantage"
	case Normal:
	}
	return "normal"
}

// ModeOf combines every source: any advantage and any disadvantage cancel out.
func ModeOf(advantages, disadvantages int) Mode {
	switch {
	case advantages > 0 && disadvantages == 0:
		return Advantage
	case disadvantages > 0 && advantages == 0:
		return Disadvantage
	}
	return Normal
}

// D20 is the attack roll's dice.
func D20(m Mode) string {
	switch m {
	case Advantage:
		return "2d20kh1"
	case Disadvantage:
		return "2d20kl1"
	case Normal:
	}
	return "1d20"
}

// HitChance is the percent chance that an attack with this bonus hits the armour class. A natural 20
// always hits and a natural 1 always misses.
func HitChance(bonus, ac int, m Mode) int {
	hits := 0
	for face := 1; face <= 20; face++ {
		if Outcome(face, bonus, ac) != Miss {
			hits++
		}
	}
	p := float64(hits) / 20
	switch m {
	case Advantage:
		p = 1 - (1-p)*(1-p)
	case Disadvantage:
		p *= p
	case Normal:
	}
	return int(math.Round(p * 100))
}

// Result is what an attack roll did.
type Result int

// Results.
const (
	Miss Result = iota
	Hit
	Critical
)

// Outcome reads the kept d20 face and the attack bonus against the armour class.
func Outcome(natural, bonus, ac int) Result {
	switch {
	case natural == 20:
		return Critical
	case natural == 1 || natural+bonus < ac:
		return Miss
	}
	return Hit
}

// Band is how far away a target is for an attack.
type Band int

// Bands.
const (
	OutOfRange Band = iota
	InReach
	InRange
	LongRange
)

// BandOf places a target at distFt: within reach, within normal range, within long range (which gives
// disadvantage), or out of range.
func BandOf(distFt, reachFt, rangeFt, longFt int) Band {
	switch {
	case distFt <= reachFt:
		return InReach
	case distFt <= rangeFt:
		return InRange
	case distFt <= longFt:
		return LongRange
	}
	return OutOfRange
}

// CriticalDice doubles every die of a damage roll.
func CriticalDice(s dice.Spec) dice.Spec {
	out := dice.Spec{Groups: make([]dice.Group, 0, len(s.Groups))}
	for _, g := range s.Groups {
		g.Count *= 2
		out.Groups = append(out.Groups, g)
	}
	return out
}

// DamageRange is the least and most damage the dice plus the bonus can deal; damage is never negative.
func DamageRange(s dice.Spec, bonus int) (least, most int) {
	least, most = bonus, bonus
	for _, g := range s.Groups {
		lo, hi := g.Count, g.Count*g.Faces
		if g.Sign == -1 {
			lo, hi = -hi, -lo
		}
		least, most = least+lo, most+hi
	}
	return max(least, 0), max(most, 0)
}

// Weapon is what a character's weapon attack depends on.
type Weapon struct {
	Finesse    bool
	Ammunition bool
	Reach      bool
}

// WeaponAttack gives a proficient character's attack bonus, damage bonus and reach with a weapon:
// Strength for melee, Dexterity with ammunition, the better of the two with finesse.
func WeaponAttack(w Weapon, strMod, dexMod, proficiency int) (toHit, damage, reachFt int) {
	mod := strMod
	switch {
	case w.Ammunition:
		mod = dexMod
	case w.Finesse:
		mod = max(strMod, dexMod)
	}
	reachFt = 5
	if w.Reach {
		reachFt = 10
	}
	return mod + proficiency, mod, reachFt
}
