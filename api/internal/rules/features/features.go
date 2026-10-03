// Package features resolves what classes, species, backgrounds and feats grant as data: Resources with
// uses that grow by level and come back on rests, values that scale with level, choices made at a level,
// and prerequisites. Nothing is special-cased by name.
package features

import (
	"slices"
	"strconv"
	"strings"
)

// Step is the value a Scale takes from a level on.
type Step[T any] struct {
	Level int
	Value T
}

// Scale is a value that grows with level: Sneak Attack dice, a Martial Arts die, Rage uses.
type Scale[T any] []Step[T]

// At is the value of the highest step reached at a level; false before the first.
func (s Scale[T]) At(level int) (T, bool) {
	var out T
	best := 0
	for _, st := range s {
		if st.Level <= level && st.Level > best {
			out, best = st.Value, st.Level
		}
	}
	return out, best > 0
}

// Event is something that gives Resources back.
type Event string

// Events.
const (
	ShortRest    Event = "short_rest"
	LongRest     Event = "long_rest"
	Dawn         Event = "dawn"
	Initiative   Event = "initiative"
	RechargeRoll Event = "roll"
)

// Basis is how a Resource's maximum is worked out.
type Basis string

// Bases.
const (
	// ByTable reads the maximum from the Resource's table by level.
	ByTable Basis = "table"
	// ByClassLevel is the class level times the multiplier (Focus Points, Lay On Hands).
	ByClassLevel Basis = "class_level"
	// ByAbility is an ability modifier, at least one (Bardic Inspiration).
	ByAbility Basis = "ability"
	// ByProficiency is the proficiency bonus.
	ByProficiency Basis = "proficiency"
)

// All gives back every expended use.
const All = 0

// Recharge is one way a Resource comes back: on an event, from a level, some or all of it. A recharge
// on a roll needs a d6 of at least RollAtLeast.
type Recharge struct {
	On          Event
	FromLevel   int
	Amount      int
	RollAtLeast int
}

// Owner is what grants something: a class, subclass, species, background or feat, by slug.
type Owner struct {
	Kind string
	Slug string
}

// Resource is a pool of uses a feature grants: Rage, Focus Points, Channel Divinity.
type Resource struct {
	Slug       string
	Name       string
	Owner      Owner
	Basis      Basis
	Multiplier int
	Ability    string
	FromLevel  int
	Table      Scale[int]
	// Die is the die a use rolls, if any, by level.
	Die       Scale[string]
	Recharges []Recharge
}

// Stats are what a Resource's maximum reads from its bearer.
type Stats struct {
	Level       int
	AbilityMod  int
	Proficiency int
}

// Max is how many uses the bearer has at most.
func (r Resource) Max(s Stats) int {
	if s.Level < r.FromLevel {
		return 0
	}
	switch r.Basis {
	case ByTable:
		n, _ := r.Table.At(s.Level)
		return n
	case ByClassLevel:
		return s.Level * r.Multiplier
	case ByAbility:
		return max(1, s.AbilityMod)
	case ByProficiency:
		return s.Proficiency
	}
	return 0
}

// DieAt is the die a use rolls at a level.
func (r Resource) DieAt(level int) (string, bool) {
	return r.Die.At(level)
}

// Regain is how many uses are left after an event: the most generous recharge that applies. A recharge
// on a roll reads the d6 rolled.
func (r Resource) Regain(e Event, s Stats, rolled, current int) int {
	most := r.Max(s)
	out := current
	for _, rc := range r.Recharges {
		if rc.On != e || s.Level < rc.FromLevel || (rc.On == RechargeRoll && rolled < rc.RollAtLeast) {
			continue
		}
		if rc.Amount == All {
			return most
		}
		out = max(out, min(most, current+rc.Amount))
	}
	return out
}

// Named is a Scale with a name and an owner: Sneak Attack belongs to the rogue.
type Named struct {
	Slug  string
	Name  string
	Owner Owner
	Steps Scale[string]
}

// Catalog is everything the rules know features grant.
type Catalog struct {
	Resources     map[string]Resource
	Scales        map[string]Named
	Choices       map[Owner][]Choice
	Prerequisites map[Owner][]Requirement
}

// ResourcesOf are the Resources an owner grants, by slug.
func (cat Catalog) ResourcesOf(o Owner) []Resource {
	var out []Resource
	for _, r := range cat.Resources {
		if r.Owner == o {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b Resource) int { return strings.Compare(a.Slug, b.Slug) })
	return out
}

// Pool is where a choice's options come from.
type Pool string

// Pools.
const (
	FeatCategory Pool = "feat_category"
	Subclass     Pool = "subclass"
	Skill        Pool = "skill"
	Expertise    Pool = "expertise"
	WeaponKind   Pool = "weapon"
	Listed       Pool = "listed"
)

// Choice is a pick a feature asks for at a level: a Fighting Style, a subclass, two Expertise skills. A
// Listed choice names its own Options.
type Choice struct {
	Slug    string
	Name    string
	Level   int
	Count   int
	Pool    Pool
	From    string
	Options []Option
}

// Option is one option a Listed choice offers.
type Option struct {
	Slug string
	Name string
}

// ChoicesAt are the choices made on reaching a level.
func ChoicesAt(cs []Choice, level int) []Choice {
	var out []Choice
	for _, c := range cs {
		if c.Level == level {
			out = append(out, c)
		}
	}
	return out
}

// Kind is what a prerequisite asks for.
type Kind string

// Kinds.
const (
	MinLevel     Kind = "level"
	MinAbility   Kind = "ability"
	Spellcasting Kind = "spellcasting"
	HasFeat      Kind = "feat"
	HasFeature   Kind = "feature"
)

// Requirement is one prerequisite. Requirements in the same group are alternatives; every group must hold.
type Requirement struct {
	Kind    Kind
	Ability string
	Minimum int
	Slug    string
	Group   int
}

// Candidate is a character as prerequisites see them.
type Candidate struct {
	Level        int
	Abilities    map[string]int
	Spellcasting bool
	Feats        []string
	Features     []string
}

// Unmet names every group of requirements the candidate does not meet, in group order; nil when all hold.
func Unmet(reqs []Requirement, c Candidate) []string {
	var groups []int
	for _, r := range reqs {
		if !slices.Contains(groups, r.Group) {
			groups = append(groups, r.Group)
		}
	}
	slices.Sort(groups)
	var out []string
	for _, g := range groups {
		var names []string
		met := false
		for _, r := range reqs {
			if r.Group == g {
				met = met || meets(r, c)
				names = append(names, name(r))
			}
		}
		if !met {
			out = append(out, strings.Join(names, " or "))
		}
	}
	return out
}

func meets(r Requirement, c Candidate) bool {
	switch r.Kind {
	case MinLevel:
		return c.Level >= r.Minimum
	case MinAbility:
		return c.Abilities[r.Ability] >= r.Minimum
	case Spellcasting:
		return c.Spellcasting
	case HasFeat:
		return slices.Contains(c.Feats, r.Slug)
	case HasFeature:
		return slices.Contains(c.Features, r.Slug)
	}
	return false
}

func name(r Requirement) string {
	switch r.Kind {
	case MinLevel:
		return "Level " + strconv.Itoa(r.Minimum) + "+"
	case MinAbility:
		return strings.ToUpper(r.Ability[:1]) + r.Ability[1:] + " " + strconv.Itoa(r.Minimum) + "+"
	case Spellcasting:
		return "Spellcasting"
	case HasFeat:
		return "the " + r.Slug + " feat"
	case HasFeature:
		return "the " + r.Slug + " feature"
	}
	return string(r.Kind)
}

// MasteryCount is how many weapons a class lets a character master at a level.
func (cat Catalog) MasteryCount(class string, level int) int {
	n, _ := cat.Scales[class+"-weapon-mastery"].Steps.At(level)
	count, _ := strconv.Atoi(n)
	return count
}
