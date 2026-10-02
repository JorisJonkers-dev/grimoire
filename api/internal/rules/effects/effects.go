// Package effects describes what spells, features and conditions do as typed components, and folds a
// creature's active effects into what attacks, saves and movement use. Nothing is special-cased by name.
package effects

import (
	"slices"
	"sort"
	"strconv"

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
// A SourceOnly edge against the bearer counts only for the Effect's source (Vex).
type Edge struct {
	Against    bool
	Advantage  bool
	Range      Range
	SourceOnly bool
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

// Incapacitated takes away the bearer's actions, bonus actions and reactions.
type Incapacitated struct{}

// Immobile holds the bearer's speed at zero.
type Immobile struct{}

// SaveMode is what a SaveEdge does to a saving throw.
type SaveMode string

// Save modes.
const (
	SaveAdvantage    SaveMode = "advantage"
	SaveDisadvantage SaveMode = "disadvantage"
	SaveFails        SaveMode = "fail"
)

// SaveEdge changes the bearer's saving throws of one ability: advantage, disadvantage or a sure failure.
type SaveEdge struct {
	Ability string
	Mode    SaveMode
}

// CritWithin makes every hit on the bearer from within Feet a Critical Hit.
type CritWithin struct {
	Feet int
}

// SpeedPenalty takes feet off the bearer's speed (Slow); several do not add up.
type SpeedPenalty struct {
	Ft int
}

// Reacts lets the bearer take a reaction when Trigger happens (damaged: a creature it can see damages
// it); the DM resolves what it does from Instruction.
type Reacts struct {
	Trigger     string
	Instruction string
}

// TempHP gives the bearer temporary hit points when the Effect lands (False Life, Heroism).
type TempHP struct {
	Amount int
}

// Teleport moves its caster up to RangeFt to an open hex (Misty Step).
type Teleport struct {
	RangeFt int
}

// ForcedMove pushes creatures that fail the save Ft feet straight away from the origin, or pulls them
// toward it (Thunderwave).
type ForcedMove struct {
	Ft     int
	Toward bool
}

// Dispel ends every spell on the bearer when the Effect lands (Dispel Magic).
type Dispel struct{}

// Counter lets the bearer counter, with its reaction, a spell cast within RangeFt (Counterspell).
type Counter struct {
	RangeFt int
}

// GrantFeature gives the bearer a feature while the Effect lasts.
type GrantFeature struct {
	Name string
}

// ResourceChange gives back (Delta above 0) or takes uses of a Resource when the Effect lands.
type ResourceChange struct {
	Resource string
	Delta    int
}

// Summon brings creatures in under the caster's control: Count of one monster, acting on the caster's
// turn (Shares) or on their own Initiative, and taking only the Dodge action unless the caster spends a
// Bonus Action to command them (NeedsCommand).
type Summon struct {
	Monster      string
	Count        int
	Shares       bool
	NeedsCommand bool
}

// Form overlays the bearer's stat block with a creature's (Polymorph, Wild Shape). Monster empty lets
// whoever applies it choose the creature. The form's hit points are Temporary Hit Points: TempHP of
// them, or the creature's Hit Point maximum when TempHP is 0. The bearer reverts when they are gone.
type Form struct {
	Monster string
	TempHP  int
}

// Reveal strips Visibility Qualities from every creature and object in the Effect's area, for
// everyone, whatever their Senses (Faerie Fire outlining the invisible).
type Reveal struct {
	Qualities []string
}

// Exhausting is exhaustion: each level takes D20PerLevel from every d20 test and SpeedFtPerLevel from
// speed, and at DeathAt levels the bearer dies.
type Exhausting struct {
	D20PerLevel     int
	SpeedFtPerLevel int
	DeathAt         int
}

func (BonusDie) isComponent()       {}
func (Incapacitated) isComponent()  {}
func (Immobile) isComponent()       {}
func (SaveEdge) isComponent()       {}
func (CritWithin) isComponent()     {}
func (Exhausting) isComponent()     {}
func (Summon) isComponent()         {}
func (Form) isComponent()           {}
func (Reveal) isComponent()         {}
func (SpeedPenalty) isComponent()   {}
func (Reacts) isComponent()         {}
func (TempHP) isComponent()         {}
func (Teleport) isComponent()       {}
func (ForcedMove) isComponent()     {}
func (Dispel) isComponent()         {}
func (Counter) isComponent()        {}
func (GrantFeature) isComponent()   {}
func (ResourceChange) isComponent() {}
func (Edge) isComponent()           {}
func (ExtraDamage) isComponent()    {}
func (MoveCost) isComponent()       {}
func (Manual) isComponent()         {}
func (Area) isComponent()           {}
func (SaveDamage) isComponent()     {}
func (SaveCondition) isComponent()  {}
func (CreateSurface) isComponent()  {}

// Definition is an Effect: what it is called, what owns it, whether it needs concentration, and what it does.
type Definition struct {
	Slug          string
	Name          string
	Owner         OwnerKind
	Concentration bool
	Duration      Duration
	Scaling       *Scaling
	Components    []Component
}

// Automated reports whether the engine computes every part of an Effect.
func (d Definition) Automated() bool {
	automated := true
	walk(d.Components, func(c Component) {
		if _, manual := c.(Manual); manual {
			automated = false
		}
	})
	return automated
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

// Active is an Effect on a creature, who put it there, how many levels of it the creature has, and the
// mode it was applied in.
type Active struct {
	Slug   string
	Source string
	Level  int
	Mode   string
}

// levels is how many times an Active Effect counts; one unless it stacks.
func (a Active) levels() int {
	return max(1, a.Level)
}

// AttackProfile is what effects do to one attack: every advantage and disadvantage with its reason,
// dice added to the attack roll and to the damage, and a note for each die.
type AttackProfile struct {
	Advantages    []string
	Disadvantages []string
	AttackDice    []string
	DamageDice    []string
	Notes         []string
	// Penalty comes off the attack roll; Crit turns a hit into a Critical Hit.
	Penalty int
	Crit    bool
}

// ForAttack folds the attacker's and the target's effects into one attack's profile.
func (cat Catalog) ForAttack(attacker, target []Active, attackerID string, withinFive bool) AttackProfile {
	var p AttackProfile
	for _, a := range attacker {
		p.attacking(cat[a.Slug], a)
	}
	for _, a := range target {
		p.attacked(cat[a.Slug], a, a.Source == attackerID, withinFive)
	}
	return p
}

// attacking applies an effect on the attacker.
func (p *AttackProfile) attacking(d Definition, a Active) {
	levels := a.levels()
	for _, c := range d.Parts(a.Mode) {
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
		case Exhausting:
			p.Penalty += c.D20PerLevel * levels
			p.Notes = append(p.Notes, d.Name+" "+strconv.Itoa(levels)+": -"+strconv.Itoa(c.D20PerLevel*levels)+" to hit")
		case ExtraDamage, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface, Incapacitated, Immobile, SaveEdge, CritWithin, SpeedPenalty, Reacts, TempHP, Teleport, ForcedMove, Dispel, Counter, GrantFeature, ResourceChange, Choice, Branch, Summon, Form, Reveal:
		}
	}
}

// attacked applies an effect on the target; extra damage only counts for the effect's own source.
func (p *AttackProfile) attacked(d Definition, a Active, bySource, withinFive bool) {
	for _, c := range d.Parts(a.Mode) {
		switch c := c.(type) {
		case Edge:
			if c.Against && c.Range.covers(withinFive) && (!c.SourceOnly || bySource) {
				p.add(d.Name, c)
			}
		case ExtraDamage:
			if bySource {
				p.DamageDice = append(p.DamageDice, c.Dice)
				p.Notes = append(p.Notes, d.Name+": +"+c.Dice+" damage")
			}
		case CritWithin:
			if withinFive && c.Feet >= 5 && !p.Crit {
				p.Crit = true
				p.Notes = append(p.Notes, d.Name+": a hit from this close is a Critical Hit")
			}
		case BonusDie, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface, Incapacitated, Immobile, SaveEdge, Exhausting, SpeedPenalty, Reacts, TempHP, Teleport, ForcedMove, Dispel, Counter, GrantFeature, ResourceChange, Choice, Branch, Summon, Form, Reveal:
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
		for _, c := range cat[a.Slug].Parts(a.Mode) {
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
		for _, c := range cat[a.Slug].Parts(a.Mode) {
			if m, ok := c.(MoveCost); ok {
				most = max(most, m.Multiplier)
			}
		}
	}
	return most
}

// has reports whether any of the bearer's effects has a component that passes the test.
func (cat Catalog) has(bearer []Active, test func(Component) bool) bool {
	for _, a := range bearer {
		if slices.ContainsFunc(cat[a.Slug].Parts(a.Mode), test) {
			return true
		}
	}
	return false
}

// Incapacitated reports whether the bearer can take no action, bonus action or reaction.
func (cat Catalog) Incapacitated(bearer []Active) bool {
	return cat.has(bearer, func(c Component) bool { _, ok := c.(Incapacitated); return ok })
}

// Immobile reports whether the bearer's speed is held at zero.
func (cat Catalog) Immobile(bearer []Active) bool {
	return cat.has(bearer, func(c Component) bool { _, ok := c.(Immobile); return ok })
}

// SaveProfile is what effects do to one saving throw: a sure failure, advantage and disadvantage with
// their reasons, dice added, and a penalty.
type SaveProfile struct {
	Fails         bool
	Advantages    []string
	Disadvantages []string
	Dice          []string
	Penalty       int
}

// ForSave folds the bearer's effects into one saving throw of an ability.
func (cat Catalog) ForSave(bearer []Active, ability string) SaveProfile {
	p := SaveProfile{Fails: false, Advantages: nil, Disadvantages: nil, Dice: cat.SaveDice(bearer), Penalty: 0}
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			switch c := c.(type) {
			case SaveEdge:
				if c.Ability != ability {
					continue
				}
				switch c.Mode {
				case SaveFails:
					p.Fails = true
				case SaveAdvantage:
					p.Advantages = append(p.Advantages, d.Name+": advantage")
				case SaveDisadvantage:
					p.Disadvantages = append(p.Disadvantages, d.Name+": disadvantage")
				}
			case Exhausting:
				p.Penalty += c.D20PerLevel * a.levels()
			case BonusDie, Edge, ExtraDamage, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface, Incapacitated, Immobile, CritWithin, SpeedPenalty, Reacts, TempHP, Teleport, ForcedMove, Dispel, Counter, GrantFeature, ResourceChange, Choice, Branch, Summon, Form, Reveal:
			}
		}
	}
	return p
}

// SpeedPenaltyFt is how many feet the bearer's effects take off its speed: exhaustion by level, plus
// the largest single SpeedPenalty.
func (cat Catalog) SpeedPenaltyFt(bearer []Active) int {
	ft, worst := 0, 0
	for _, a := range bearer {
		for _, c := range cat[a.Slug].Parts(a.Mode) {
			switch c := c.(type) {
			case Exhausting:
				ft += c.SpeedFtPerLevel * a.levels()
			case SpeedPenalty:
				worst = max(worst, c.Ft)
			case BonusDie, Edge, ExtraDamage, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface, Incapacitated, Immobile, SaveEdge, CritWithin, Reacts, TempHP, Teleport, ForcedMove, Dispel, Counter, GrantFeature, ResourceChange, Choice, Branch, Summon, Form, Reveal:
			}
		}
	}
	return ft + worst
}

// Fatal names the effect that kills the bearer at its level, if any (six levels of exhaustion).
func (cat Catalog) Fatal(bearer []Active) (string, bool) {
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if e, ok := c.(Exhausting); ok && e.DeathAt > 0 && a.levels() >= e.DeathAt {
				return d.Name, true
			}
		}
	}
	return "", false
}

// Reaction is a reaction an Effect gives: the Effect's name and what the DM resolves.
type Reaction struct {
	Name        string
	Instruction string
}

// ReactionsTo lists the reactions the bearer's effects give it when a trigger happens.
func (cat Catalog) ReactionsTo(bearer []Active, trigger string) []Reaction {
	var out []Reaction
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if r, ok := c.(Reacts); ok && r.Trigger == trigger {
				out = append(out, Reaction{Name: d.Name, Instruction: r.Instruction})
			}
		}
	}
	return out
}

// Landing is what an Effect does the moment it lands on its bearer.
type Landing struct {
	TempHP    int
	Dispels   bool
	Grants    []string
	Resources []ResourceChange
	Form      *Form
	Reveals   []string
}

// LandingOf is what an Effect does as it lands.
func (cat Catalog) LandingOf(slug, mode string) Landing {
	var out Landing
	for _, c := range cat[slug].Parts(mode) {
		switch c := c.(type) {
		case TempHP:
			out.TempHP = max(out.TempHP, c.Amount)
		case Dispel:
			out.Dispels = true
		case GrantFeature:
			out.Grants = append(out.Grants, c.Name)
		case Reveal:
			out.Reveals = append(out.Reveals, c.Qualities...)
		case Form:
			out.Form = &c
		case ResourceChange:
			out.Resources = append(out.Resources, c)
		case BonusDie, Edge, ExtraDamage, MoveCost, Manual, Area, SaveDamage, SaveCondition, CreateSurface, Incapacitated, Immobile, SaveEdge,
			CritWithin, Exhausting, SpeedPenalty, Reacts, Teleport, ForcedMove, Counter, Choice, Branch, Summon:
		}
	}
	return out
}

// TeleportOf is how far an Effect teleports its caster, if it does.
func (cat Catalog) TeleportOf(slug string) (int, bool) {
	for _, c := range cat[slug].Components {
		if t, ok := c.(Teleport); ok {
			return t.RangeFt, true
		}
	}
	return 0, false
}

// CounterOf is the longest counter among the bearer's effects, with its Effect's name.
func (cat Catalog) CounterOf(bearer []Active) (int, string, bool) {
	best, name := 0, ""
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if k, ok := c.(Counter); ok && k.RangeFt > best {
				best, name = k.RangeFt, d.Name
			}
		}
	}
	return best, name, best > 0
}

// SummonOf is what an Effect summons, if it does.
func (cat Catalog) SummonOf(slug string) (Summon, bool) {
	for _, c := range cat[slug].Components {
		if s, ok := c.(Summon); ok {
			return s, true
		}
	}
	return Summon{Monster: "", Count: 0, Shares: false, NeedsCommand: false}, false
}

// Spell reports whether an Effect belongs to a spell, the kind Dispel ends.
func (cat Catalog) Spell(slug string) bool {
	return cat[slug].Owner == OwnedBySpell
}

// Stacks reports whether applying the Effect again adds a level instead of a second copy.
func (cat Catalog) Stacks(slug string) bool {
	return slices.ContainsFunc(cat[slug].Components, func(c Component) bool { _, ok := c.(Exhausting); return ok })
}

// Instructions are the parts of an Effect the DM resolves by hand; an unknown Effect is one whole
// instruction, so nothing is ever skipped silently.
func (cat Catalog) Instructions(slug, name, mode string) []string {
	d, known := cat[slug]
	if !known {
		return []string{"Resolve " + name + " by hand."}
	}
	var out []string
	for _, c := range d.Parts(mode) {
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
	Push         ForcedMove
	Reveals      []string
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
		case Reveal:
			out.Reveals = append(out.Reveals, c.Qualities...)
		case ForcedMove:
			out.Push = c
		case BonusDie, Edge, ExtraDamage, MoveCost, Incapacitated, Immobile, SaveEdge, CritWithin, Exhausting, SpeedPenalty, Reacts, TempHP, Teleport, Dispel, Counter, GrantFeature, ResourceChange, Choice, Branch, Summon, Form:
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
