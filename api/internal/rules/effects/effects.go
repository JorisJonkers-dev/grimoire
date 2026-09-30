// Package effects describes what spells, features and conditions do as typed components, and folds a
// creature's active effects into what attacks, saves and movement use. Nothing is special-cased by name.
package effects

import (
	"slices"
	"sort"
)

// Component is one typed part of an Effect.
//
//sumtype:decl
type Component interface{ isComponent() }

// Roll is a kind of d20 roll a bonus die can join.
type Roll int

// Rolls.
const (
	AttackRolls Roll = iota
	SavingThrows
)

// Range limits an advantage source by how far the attacker is from the bearer.
type Range int

// Ranges.
const (
	AnyRange Range = iota
	WithinFive
	BeyondFive
)

// BonusDie adds a die to the bearer's rolls of some kinds (Bless).
type BonusDie struct {
	On   []Roll
	Dice string
}

// Edge gives advantage or disadvantage: on the bearer's own attacks, or on attacks against the bearer.
type Edge struct {
	Against   bool
	Advantage bool
	Range     Range
}

// ExtraDamage adds dice to hits against the bearer by the effect's source (Hunter's Mark).
type ExtraDamage struct {
	Dice string
}

// MoveCost multiplies what each foot of movement costs the bearer (crawling while prone).
type MoveCost struct {
	Multiplier int
}

// Manual is a part the engine cannot compute yet; the DM resolves it from a prompt.
type Manual struct {
	Instruction string
}

func (BonusDie) isComponent()    {}
func (Edge) isComponent()        {}
func (ExtraDamage) isComponent() {}
func (MoveCost) isComponent()    {}
func (Manual) isComponent()      {}

// Definition is an Effect: what it is called, whether it needs concentration, and what it does.
type Definition struct {
	Slug          string
	Name          string
	Concentration bool
	Components    []Component
}

// Automated reports whether the engine computes every part of an Effect.
func (d Definition) Automated() bool {
	for _, c := range d.Components {
		if _, manual := c.(Manual); manual {
			return false
		}
	}
	return true
}

// Active is an Effect on a creature, and who put it there.
type Active struct {
	Slug   string
	Source string
}

// AttackProfile is what effects do to one attack: every advantage and disadvantage with its reason,
// dice added to the attack roll and to the damage, and a note for each die.
type AttackProfile struct {
	Advantages    []string
	Disadvantages []string
	AttackDice    []string
	DamageDice    []string
	Notes         []string
}

// ForAttack folds the attacker's and the target's effects into one attack's profile.
func ForAttack(attacker, target []Active, attackerID string, withinFive bool) AttackProfile {
	var p AttackProfile
	for _, a := range attacker {
		p.attacking(lookup(a.Slug))
	}
	for _, a := range target {
		p.attacked(lookup(a.Slug), a.Source == attackerID, withinFive)
	}
	return p
}

// attacking applies an effect on the attacker.
func (p *AttackProfile) attacking(d Definition) {
	for _, c := range d.Components {
		switch c := c.(type) {
		case BonusDie:
			if slices.Contains(c.On, AttackRolls) {
				p.AttackDice = append(p.AttackDice, c.Dice)
				p.Notes = append(p.Notes, d.Name+": +"+c.Dice+" to hit")
			}
		case Edge:
			if !c.Against {
				p.add(d.Name, c)
			}
		case ExtraDamage, MoveCost, Manual:
		}
	}
}

// attacked applies an effect on the target; extra damage only counts for the effect's own source.
func (p *AttackProfile) attacked(d Definition, bySource, withinFive bool) {
	for _, c := range d.Components {
		switch c := c.(type) {
		case Edge:
			if c.Against && c.Range.covers(withinFive) {
				p.add(d.Name, c)
			}
		case ExtraDamage:
			if bySource {
				p.DamageDice = append(p.DamageDice, c.Dice)
				p.Notes = append(p.Notes, d.Name+": +"+c.Dice+" damage")
			}
		case BonusDie, MoveCost, Manual:
		}
	}
}

func (r Range) covers(withinFive bool) bool {
	switch r {
	case WithinFive:
		return withinFive
	case BeyondFive:
		return !withinFive
	case AnyRange:
	}
	return true
}

func (p *AttackProfile) add(name string, e Edge) {
	if e.Advantage {
		p.Advantages = append(p.Advantages, name+": advantage")
		return
	}
	p.Disadvantages = append(p.Disadvantages, name+": disadvantage")
}

// SaveDice are the dice a creature's effects add to its saving throws.
func SaveDice(bearer []Active) []string {
	var out []string
	for _, a := range bearer {
		for _, c := range lookup(a.Slug).Components {
			if b, ok := c.(BonusDie); ok && slices.Contains(b.On, SavingThrows) {
				out = append(out, b.Dice)
			}
		}
	}
	return out
}

// MoveMultiplier is what each foot of movement costs a creature: the steepest of its effects.
func MoveMultiplier(bearer []Active) int {
	most := 1
	for _, a := range bearer {
		for _, c := range lookup(a.Slug).Components {
			if m, ok := c.(MoveCost); ok {
				most = max(most, m.Multiplier)
			}
		}
	}
	return most
}

// Instructions are the parts of an Effect the DM resolves by hand; an unknown Effect is one whole
// instruction, so nothing is ever skipped silently.
func Instructions(slug, name string) []string {
	d, known := catalog()[slug]
	if !known {
		return []string{"Resolve " + name + " by hand."}
	}
	var out []string
	for _, c := range d.Components {
		if m, ok := c.(Manual); ok {
			out = append(out, m.Instruction)
		}
	}
	return out
}

// Lookup finds a modelled Effect.
func Lookup(slug string) (Definition, bool) {
	d, ok := catalog()[slug]
	return d, ok
}

// Automated lists the slugs of every Effect the engine computes in full.
func Automated() []string {
	return slugs(true)
}

// Partial lists the slugs of Effects the engine computes in part, leaving the rest to the DM.
func Partial() []string {
	return slugs(false)
}

func slugs(automated bool) []string {
	var out []string
	for slug, d := range catalog() {
		if d.Automated() == automated {
			out = append(out, slug)
		}
	}
	sort.Strings(out)
	return out
}

func lookup(slug string) Definition {
	return catalog()[slug]
}

// catalog is every Effect the engine models, keyed by compendium slug.
func catalog() map[string]Definition {
	return map[string]Definition{
		"bless": {Slug: "bless", Name: "Bless", Concentration: true, Components: []Component{
			BonusDie{On: []Roll{AttackRolls, SavingThrows}, Dice: "1d4"},
		}},
		"faerie-fire": {Slug: "faerie-fire", Name: "Faerie Fire", Concentration: true, Components: []Component{
			Edge{Against: true, Advantage: true, Range: AnyRange},
		}},
		"hunters-mark": {Slug: "hunters-mark", Name: "Hunter's Mark", Concentration: true, Components: []Component{
			ExtraDamage{Dice: "1d6"},
		}},
		"prone": {Slug: "prone", Name: "Prone", Concentration: false, Components: []Component{
			Edge{Against: false, Advantage: false, Range: AnyRange},
			Edge{Against: true, Advantage: true, Range: WithinFive},
			Edge{Against: true, Advantage: false, Range: BeyondFive},
			MoveCost{Multiplier: 2},
		}},
		"poisoned": {Slug: "poisoned", Name: "Poisoned", Concentration: false, Components: []Component{
			Edge{Against: false, Advantage: false, Range: AnyRange},
			Manual{Instruction: "Poisoned: ability checks are made with disadvantage."},
		}},
	}
}
