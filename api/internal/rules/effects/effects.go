// Package effects describes what spells, features and conditions do as typed components, and folds a
// creature's active effects into what attacks, saves and movement use. Nothing is special-cased by name.
package effects

import (
	"slices"
	"sort"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
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

// Area is an effect's template; RangeFt is how far away its point may be, 0 when it starts at the caster.
type Area struct {
	Shape   hex.Shape
	SizeFt  int
	RangeFt int
}

// SaveDamage hurts every creature in the area; a successful save halves it with Half, or avoids it.
type SaveDamage struct {
	Ability string
	Dice    string
	Type    string
	Half    bool
}

// SaveCondition puts a condition on every creature in the area that fails the save.
type SaveCondition struct {
	Ability string
	Slug    string
}

// CreateSurface leaves a Surface on the area for some rounds.
type CreateSurface struct {
	Kind   surface.Kind
	Rounds int
}

func (BonusDie) isComponent()      {}
func (Edge) isComponent()          {}
func (ExtraDamage) isComponent()   {}
func (MoveCost) isComponent()      {}
func (Manual) isComponent()        {}
func (Area) isComponent()          {}
func (SaveDamage) isComponent()    {}
func (SaveCondition) isComponent() {}
func (CreateSurface) isComponent() {}

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

// Catalog is every Effect the engine knows, keyed by slug.
type Catalog map[string]Definition

// OwnerKind is what an Effect belongs to; every Effect has exactly one owner.
type OwnerKind string

// Owner kinds.
const (
	OwnedBySpell           OwnerKind = "spell"
	OwnedByFeature         OwnerKind = "feature"
	OwnedByItemProperty    OwnerKind = "item_property"
	OwnedByCondition       OwnerKind = "condition"
	OwnedByMonsterAction   OwnerKind = "monster_action"
	OwnedBySurface         OwnerKind = "surface"
	OwnedByRuleVariant     OwnerKind = "rule_variant"
	OwnedByTrap            OwnerKind = "trap"
	OwnedByRollTableResult OwnerKind = "roll_table_result"
)

// Owner is the thing an Effect belongs to, by kind and slug.
type Owner struct {
	Kind OwnerKind
	Slug string
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
func (cat Catalog) ForAttack(attacker, target []Active, attackerID string, withinFive bool) AttackProfile {
	var p AttackProfile
	for _, a := range attacker {
		p.attacking(cat[a.Slug])
	}
	for _, a := range target {
		p.attacked(cat[a.Slug], a.Source == attackerID, withinFive)
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
		case ExtraDamage, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface:
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
		case BonusDie, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface:
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
func (cat Catalog) SaveDice(bearer []Active) []string {
	var out []string
	for _, a := range bearer {
		for _, c := range cat[a.Slug].Components {
			if b, ok := c.(BonusDie); ok && slices.Contains(b.On, SavingThrows) {
				out = append(out, b.Dice)
			}
		}
	}
	return out
}

// MoveMultiplier is what each foot of movement costs a creature: the steepest of its effects.
func (cat Catalog) MoveMultiplier(bearer []Active) int {
	most := 1
	for _, a := range bearer {
		for _, c := range cat[a.Slug].Components {
			if m, ok := c.(MoveCost); ok {
				most = max(most, m.Multiplier)
			}
		}
	}
	return most
}

// Instructions are the parts of an Effect the DM resolves by hand; an unknown Effect is one whole
// instruction, so nothing is ever skipped silently.
func (cat Catalog) Instructions(slug, name string) []string {
	d, known := cat[slug]
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

// AreaSpell is an area Effect laid out for casting.
type AreaSpell struct {
	Name         string
	Area         Area
	Save         string
	Damage       SaveDamage
	Condition    string
	Surface      CreateSurface
	Instructions []string
}

// AreaOf finds a modelled area Effect.
func (cat Catalog) AreaOf(slug string) (AreaSpell, bool) {
	d, known := cat[slug]
	var out AreaSpell
	out.Name = d.Name
	found := false
	for _, c := range d.Components {
		switch c := c.(type) {
		case Area:
			out.Area, found = c, true
		case SaveDamage:
			out.Damage, out.Save = c, c.Ability
		case SaveCondition:
			out.Condition, out.Save = c.Slug, c.Ability
		case CreateSurface:
			out.Surface = c
		case Manual:
			out.Instructions = append(out.Instructions, c.Instruction)
		case BonusDie, Edge, ExtraDamage, MoveCost:
		}
	}
	return out, known && found
}

// Lookup finds a modelled Effect.
func (cat Catalog) Lookup(slug string) (Definition, bool) {
	d, ok := cat[slug]
	return d, ok
}

// Automated lists the slugs of every Effect the engine computes in full.
func (cat Catalog) Automated() []string {
	return cat.slugs(true)
}

// Partial lists the slugs of Effects the engine computes in part, leaving the rest to the DM.
func (cat Catalog) Partial() []string {
	return cat.slugs(false)
}

func (cat Catalog) slugs(automated bool) []string {
	var out []string
	for slug, d := range cat {
		if d.Automated() == automated {
			out = append(out, slug)
		}
	}
	sort.Strings(out)
	return out
}
