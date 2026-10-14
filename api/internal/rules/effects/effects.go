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
type Component interface {
	isComponent()
	// Kind is the slug of the Part's kind, the one its definition in Kinds describes.
	Kind() string
	// Text is the Part's rules sentence.
	Text() string
}

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
		if len(prompts(c)) > 0 {
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
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if f, ok := c.(AttackFolder); ok {
				f.FoldAttacking(&p, a.held(d))
			}
		}
	}
	for _, a := range target {
		d := cat[a.Slug]
		by := Attacker{Source: a.Source == attackerID, WithinFive: withinFive}
		for _, c := range d.Parts(a.Mode) {
			if f, ok := c.(AttackedFolder); ok {
				f.FoldAttacked(&p, a.held(d), by)
			}
		}
	}
	return p
}

// held is the Effect as its bearer holds it.
func (a Active) held(d Definition) Held {
	return Held{Name: d.Name, Levels: a.levels()}
}

// SaveDice are the dice a creature's effects add to its saving throws, whatever the ability.
func (cat Catalog) SaveDice(bearer []Active) []string {
	return cat.ForSave(bearer, "").Dice
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
	p := SaveProfile{Fails: false, Advantages: nil, Disadvantages: nil, Dice: nil, Penalty: 0}
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if f, ok := c.(SaveFolder); ok {
				f.FoldSave(&p, a.held(d), ability)
			}
		}
	}
	return p
}

// MoveProfile is what effects do to how a creature moves: speed held at zero, what each foot costs,
// and the feet taken off its speed.
type MoveProfile struct {
	Immobile bool
	// Multiplier is what each foot costs: the steepest of the effects, 1 with none.
	Multiplier int
	// StackedFt adds up across effects and levels; LargestFt is the largest single penalty, since those
	// do not add up.
	StackedFt int
	LargestFt int
}

// ForMove folds the bearer's effects into how it moves.
func (cat Catalog) ForMove(bearer []Active) MoveProfile {
	p := MoveProfile{Immobile: false, Multiplier: 1, StackedFt: 0, LargestFt: 0}
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if f, ok := c.(MoveFolder); ok {
				f.FoldMove(&p, a.held(d))
			}
		}
	}
	return p
}

// MoveMultiplier is what each foot of movement costs a creature: the steepest of its effects.
func (cat Catalog) MoveMultiplier(bearer []Active) int {
	return cat.ForMove(bearer).Multiplier
}

// Immobile reports whether the bearer's speed is held at zero.
func (cat Catalog) Immobile(bearer []Active) bool {
	return cat.ForMove(bearer).Immobile
}

// SpeedPenaltyFt is how many feet the bearer's effects take off its speed: exhaustion by level, plus
// the largest single SpeedPenalty.
func (cat Catalog) SpeedPenaltyFt(bearer []Active) int {
	p := cat.ForMove(bearer)
	return p.StackedFt + p.LargestFt
}

// ActionProfile is what effects do to what a creature can do: whether it can act at all, the reactions
// they give it by what triggers them, and the longest counter with the Effect that gives it.
type ActionProfile struct {
	Incapacitated bool
	Reactions     map[string][]Reaction
	CounterFt     int
	CounterBy     string
}

// ForActions folds the bearer's effects into what it can do.
func (cat Catalog) ForActions(bearer []Active) ActionProfile {
	p := ActionProfile{Incapacitated: false, Reactions: nil, CounterFt: 0, CounterBy: ""}
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if f, ok := c.(ActionFolder); ok {
				f.FoldActions(&p, a.held(d))
			}
		}
	}
	return p
}

// Incapacitated reports whether the bearer can take no action, bonus action or reaction.
func (cat Catalog) Incapacitated(bearer []Active) bool {
	return cat.ForActions(bearer).Incapacitated
}

// Reaction is a reaction an Effect gives: the Effect's name and what the DM resolves.
type Reaction struct {
	Name        string
	Instruction string
}

// ReactionsTo lists the reactions the bearer's effects give it when a trigger happens.
func (cat Catalog) ReactionsTo(bearer []Active, trigger string) []Reaction {
	return cat.ForActions(bearer).Reactions[trigger]
}

// CounterOf is the longest counter among the bearer's effects, with its Effect's name.
func (cat Catalog) CounterOf(bearer []Active) (int, string, bool) {
	p := cat.ForActions(bearer)
	return p.CounterFt, p.CounterBy, p.CounterFt > 0
}

// Fatal names the effect that kills the bearer at its level, if any (six levels of exhaustion).
func (cat Catalog) Fatal(bearer []Active) (string, bool) {
	for _, a := range bearer {
		d := cat[a.Slug]
		for _, c := range d.Parts(a.Mode) {
			if s, ok := c.(Stacker); ok && s.Kills(a.levels()) {
				return d.Name, true
			}
		}
	}
	return "", false
}

// Stacks reports whether applying the Effect again adds a level instead of a second copy.
func (cat Catalog) Stacks(slug string) bool {
	return slices.ContainsFunc(cat[slug].Components, func(c Component) bool { _, ok := c.(Stacker); return ok })
}

// MaxLevel is the highest level a stacking Effect rises to; 0 when it has no cap or does not stack.
func (cat Catalog) MaxLevel(slug string) int {
	for _, c := range cat[slug].Components {
		if s, ok := c.(Stacker); ok {
			return s.Cap()
		}
	}
	return 0
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
		for _, act := range landsOnBearer(c) {
			out.take(act)
		}
	}
	return out
}

// landsOnBearer is what a Part does on its bearer as its Effect lands; nothing for most.
func landsOnBearer(c Component) []Act {
	if l, ok := c.(BearerLander); ok {
		return l.LandsOnBearer()
	}
	return nil
}

// take applies one act to the bearer: temporary hit points keep the highest, the last form holds,
// and the rest add up.
func (l *Landing) take(act Act) {
	switch act := act.(type) {
	case GainTempHP:
		l.TempHP = max(l.TempHP, act.Amount)
	case EndSpells:
		l.Dispels = true
	case Grant:
		l.Grants = append(l.Grants, act.Feature)
	case ChangeUses:
		l.Resources = append(l.Resources, ResourceChange(act))
	case StripQualities:
		l.Reveals = append(l.Reveals, act.Qualities...)
	case TakeForm:
		f := Form(act)
		l.Form = &f
	case TeleportCaster, SummonCreatures, Afflict, AskDM, RollSave, RollDamage, Hurt, Template, LaySurface, Push, ShedLight:
	}
}

// TeleportOf is how far an Effect teleports its caster, if it does.
func (cat Catalog) TeleportOf(slug string) (int, bool) {
	for _, c := range cat[slug].Components {
		for _, act := range landsOnBearer(c) {
			if t, ok := act.(TeleportCaster); ok {
				return t.RangeFt, true
			}
		}
	}
	return 0, false
}

// SummonOf is what an Effect summons, if it does.
func (cat Catalog) SummonOf(slug string) (Summon, bool) {
	for _, c := range cat[slug].Components {
		for _, act := range landsOnBearer(c) {
			if s, ok := act.(SummonCreatures); ok {
				return Summon(s), true
			}
		}
	}
	return Summon{Monster: "", Count: 0, Shares: false, NeedsCommand: false}, false
}

// prompts are the instructions a Part leaves the DM as its Effect lands.
func prompts(c Component) []string {
	var out []string
	for _, act := range landsOnBearer(c) {
		if a, ok := act.(AskDM); ok {
			out = append(out, a.Instruction)
		}
	}
	return out
}

// Spell reports whether an Effect belongs to a spell, the kind Dispel ends.
func (cat Catalog) Spell(slug string) bool {
	return cat[slug].Owner == OwnedBySpell
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
		out = append(out, prompts(c)...)
	}
	return out
}

// AreaSpell is an area Effect laid out for casting.
type AreaSpell struct {
	Name         string
	Area         Area
	Save         string
	Damage       RollDamage
	Condition    string
	Surface      CreateSurface
	Push         ForcedMove
	Reveals      []string
	Instructions []string
	// Light is the light the spell sheds where it lands, zero for none.
	Light   Light
	shaped  bool
	landers []Lander
}

// ask takes what an area Part asks for into the area's plan: the later of two settings wins, and
// reveals and instructions add up. A Part that lands is kept to land later.
func (a *AreaSpell) ask(p Asker) {
	for _, act := range p.Asks() {
		switch act := act.(type) {
		case Template:
			a.Area, a.shaped = act.Area, true
		case RollSave:
			a.Save = act.Ability
		case RollDamage:
			a.Damage = act
		case Afflict:
			a.Condition = act.Condition
		case LaySurface:
			a.Surface = CreateSurface(act)
		case Push:
			a.Push = ForcedMove(act)
		case StripQualities:
			a.Reveals = append(a.Reveals, act.Qualities...)
		case AskDM:
			a.Instructions = append(a.Instructions, act.Instruction)
		case ShedLight:
			a.Light = Light(act)
		case Hurt, GainTempHP, EndSpells, Grant, ChangeUses, TeleportCaster, TakeForm, SummonCreatures:
		}
	}
	if l, ok := p.(Lander); ok {
		a.landers = append(a.landers, l)
	}
}

// Lands is what the area's Parts do to one creature once its rolls are in.
func (a AreaSpell) Lands(o Outcome) []Act {
	var out []Act
	for _, l := range a.landers {
		out = append(out, l.Lands(o)...)
	}
	return out
}

// AreaOf finds a modelled area Effect.
func (cat Catalog) AreaOf(slug string) (AreaSpell, bool) {
	d, known := cat[slug]
	var out AreaSpell
	out.Name = d.Name
	for _, c := range d.Components {
		if a, ok := c.(Asker); ok {
			out.ask(a)
		}
	}
	return out, known && out.shaped
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
