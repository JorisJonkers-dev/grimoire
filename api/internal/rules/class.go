package rules

import (
	"slices"
	"strings"
)

// Kinds of homebrew spellcasting beside the SRD's.
const (
	// CustomSlots reads spell slots from the class's own table.
	CustomSlots CasterKind = "slots"
	// SpellPoints casts from a pool of points instead of slots.
	SpellPoints CasterKind = "points"
)

// Spellcasting is how a class casts: its kind, cantrips and prepared spells by class level, its own
// slot table or spell points and their costs by spell level, and the highest spell it reaches.
type Spellcasting struct {
	Kind     CasterKind
	Cantrips [20]int
	Prepared [20]int
	Slots    [20][9]int
	Points   [20]int
	Costs    [9]int
	MaxSpell [20]int
	// Spellbook prepares from a book spells are copied into; AfterRest changes prepared spells after
	// every long rest instead of swapping one a level.
	Spellbook bool
	AfterRest bool
}

// Class is what the rules read of a class: its primary abilities (AnyPrimary takes any one for
// multiclassing), spellcasting, training and how many skills it picks. SRD classes come from SRD;
// homebrew ones from their design.
type Class struct {
	Slug          string
	Name          string
	Primary       []Ability
	AnyPrimary    bool
	Casting       Spellcasting
	Proficiencies Proficiencies
	Skills        int
}

// Classes are the SRD classes, by slug.
func Classes() []string {
	return []string{"barbarian", "bard", "cleric", "druid", "fighter", "monk", "paladin", "ranger", "rogue", "sorcerer", "warlock", "wizard"}
}

// SRD is an SRD class's profile; a class it does not know casts nothing and trains nothing.
func SRD(slug string) Class {
	casting := Spellcasting{
		Kind: srdCaster(slug), Cantrips: [20]int{}, Prepared: preparedTables()[slug], Slots: [20][9]int{}, Points: [20]int{}, Costs: [9]int{},
		MaxSpell: [20]int{}, Spellbook: slug == "wizard", AfterRest: slices.Contains([]string{"cleric", "druid", "paladin", "wizard"}, slug),
	}
	for l := range 20 {
		casting.Cantrips[l] = srdCantrips(slug, l+1)
	}
	return Class{
		Slug: slug, Name: title(slug), Primary: srdPrimary(slug), AnyPrimary: slug == "fighter", Casting: casting,
		Proficiencies: srdProficiencies(slug), Skills: srdSkillCount(slug),
	}
}

func title(slug string) string {
	if slug == "" {
		return ""
	}
	return strings.ToUpper(slug[:1]) + slug[1:]
}

func level20(level int) int { return min(max(level, 1), 20) }

// Caster reports whether the class casts spells at all.
func (c Class) Caster() bool { return c.Casting.Kind != NoCaster && c.Casting.Kind != "" }

// CantripsAt is how many cantrips the class knows at a class level.
func (c Class) CantripsAt(level int) int { return c.Casting.Cantrips[level20(level)-1] }

// PreparedAt is how many spells of level 1 and up the class prepares at a class level.
func (c Class) PreparedAt(level int) int { return c.Casting.Prepared[level20(level)-1] }

// MaxSpellLevel is the highest level of spell the class casts at a class level; 0 without spellcasting.
func (c Class) MaxSpellLevel(level int) int {
	level = level20(level)
	switch c.Casting.Kind {
	case FullCaster:
		return min((level+1)/2, 9)
	case HalfCaster:
		return (level + 3) / 4
	case PactCaster:
		return min((level+1)/2, 5)
	case CustomSlots:
		top := 0
		for i, n := range c.Casting.Slots[level-1] {
			if n > 0 {
				top = i + 1
			}
		}
		return top
	case SpellPoints:
		return c.Casting.MaxSpell[level-1]
	case NoCaster:
	}
	return 0
}

// SlotsAt are the spell slots per spell level the class has at a class level: half casters (who cast
// from level 1 in SRD 5.2) as a full caster of half their level rounded up, pact casters a few slots
// of one level, a custom table its own row. Spell points have none.
func (c Class) SlotsAt(level int) [9]int {
	level = level20(level)
	switch c.Casting.Kind {
	case FullCaster:
		return fullCasterSlots[level-1]
	case HalfCaster:
		return fullCasterSlots[(level+1)/2-1]
	case PactCaster:
		var out [9]int
		out[min((level+1)/2, 5)-1] = pactSlots(level)
		return out
	case CustomSlots:
		return c.Casting.Slots[level-1]
	case SpellPoints, NoCaster:
	}
	return [9]int{}
}

// PointCost is what a spell of a level costs a spell-point caster at a class level: up to the highest
// spell it reaches, and never a cantrip.
func PointCost(c Class, level, spellLevel int) (int, error) {
	switch {
	case c.Casting.Kind != SpellPoints:
		return 0, violation("%s does not cast with spell points", c.Name)
	case spellLevel < 1 || spellLevel > c.MaxSpellLevel(level):
		return 0, violation("%s casts spells of level 1 to %d with spell points", c.Name, c.MaxSpellLevel(level))
	}
	return c.Casting.Costs[spellLevel-1], nil
}

// SpendPoints is the spell points left after casting a spell of a level from the points there are.
func SpendPoints(c Class, level, current, spellLevel int) (int, error) {
	cost, err := PointCost(c, level, spellLevel)
	if err != nil {
		return current, err
	}
	if cost > current {
		return current, violation("a level %d spell needs %d spell points; %d are left", spellLevel, cost, current)
	}
	return current - cost, nil
}

func srdCaster(slug string) CasterKind {
	switch slug {
	case "bard", "cleric", "druid", "sorcerer", "wizard":
		return FullCaster
	case "paladin", "ranger":
		return HalfCaster
	case "warlock":
		return PactCaster
	default:
		return NoCaster
	}
}

func srdPrimary(slug string) []Ability {
	switch slug {
	case "barbarian":
		return []Ability{Strength}
	case "bard", "sorcerer", "warlock":
		return []Ability{Charisma}
	case "cleric", "druid":
		return []Ability{Wisdom}
	case "fighter":
		return []Ability{Strength, Dexterity}
	case "monk", "ranger":
		return []Ability{Dexterity, Wisdom}
	case "paladin":
		return []Ability{Strength, Charisma}
	case "rogue":
		return []Ability{Dexterity}
	case "wizard":
		return []Ability{Intelligence}
	default:
		return nil
	}
}

func srdSkillCount(slug string) int {
	switch slug {
	case "rogue":
		return 4
	case "bard", "ranger":
		return 3
	default:
		return 2
	}
}

func srdCantrips(slug string, level int) int {
	base := map[string]int{"bard": 2, "cleric": 3, "druid": 2, "sorcerer": 4, "warlock": 2, "wizard": 3}[slug]
	if base == 0 {
		return 0
	}
	switch {
	case level >= 10:
		return base + 2
	case level >= 4:
		return base + 1
	default:
		return base
	}
}

func srdProficiencies(slug string) Proficiencies {
	simple, martial := "Simple weapons", "Martial weapons"
	switch slug {
	case "barbarian", "ranger":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Shields"}, Weapons: []string{simple, martial}}
	case "fighter", "paladin":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Heavy armor", "Shields"}, Weapons: []string{simple, martial}}
	case "cleric", "druid":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Shields"}, Weapons: []string{simple}}
	case "bard", "warlock":
		return Proficiencies{Armor: []string{"Light armor"}, Weapons: []string{simple}}
	case "rogue":
		return Proficiencies{Armor: []string{"Light armor"}, Weapons: []string{simple, "Martial weapons with the Finesse or Light property"}}
	case "monk":
		return Proficiencies{Armor: nil, Weapons: []string{simple, "Martial weapons with the Light property"}}
	case "sorcerer", "wizard":
		return Proficiencies{Armor: nil, Weapons: []string{simple}}
	default:
		return Proficiencies{Armor: nil, Weapons: nil}
	}
}
