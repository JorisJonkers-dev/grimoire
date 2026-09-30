package rules

import "strconv"

// CasterKind is how a class casts spells.
type CasterKind string

// Caster kinds.
const (
	NoCaster   CasterKind = "none"
	FullCaster CasterKind = "full"
	HalfCaster CasterKind = "half"
	PactCaster CasterKind = "pact"
)

// CasterFor is the SRD caster kind of a class.
func CasterFor(class string) CasterKind {
	switch class {
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

// Armor is worn body armour.
type Armor struct {
	Base   int
	AddDex bool
	// DexCap limits the Dexterity bonus; negative means no cap.
	DexCap           int
	StrengthRequired int
	Stealth          bool
}

// ArmorClass is the AC from Dexterity, optional body armour and a shield bonus (0 without one).
func ArmorClass(dexMod int, armor *Armor, shield int) int {
	if armor == nil {
		return 10 + dexMod + shield
	}
	dex := 0
	if armor.AddDex {
		dex = dexMod
		if armor.DexCap >= 0 && dex > armor.DexCap {
			dex = armor.DexCap
		}
	}
	return armor.Base + dex + shield
}

// FirstLevelHP is maximum hit points at level 1: the hit die's maximum plus the Constitution modifier.
func FirstLevelHP(hitDie, conMod int) int {
	return max(1, hitDie+conMod)
}

// Resource is a spendable pool such as hit dice or spell slots.
type Resource struct {
	Key     string
	Label   string
	Current int
	Max     int
}

// FirstLevelResources are the pools a first-level character starts with.
func FirstLevelResources(class string, hitDie, rulesetYear int) []Resource {
	out := []Resource{{Key: "hit-dice", Label: "Hit Dice (d" + strconv.Itoa(hitDie) + ")", Current: 1, Max: 1}}
	slots := 0
	switch CasterFor(class) {
	case FullCaster:
		slots = 2
	case HalfCaster:
		if rulesetYear >= 2024 {
			slots = 2
		}
	case PactCaster:
		slots = 1
	case NoCaster:
	}
	if slots > 0 {
		out = append(out, Resource{Key: "spell-slots-1", Label: "Level 1 spell slots", Current: slots, Max: slots})
	}
	return out
}

// Save is a saving throw on the sheet.
type Save struct {
	Ability    Ability
	Score      int
	Modifier   int
	Proficient bool
	Bonus      int
}

// SkillBonus is a skill on the sheet.
type SkillBonus struct {
	Skill      Skill
	Ability    Ability
	Proficient bool
	Bonus      int
}

// SheetInput is everything the sheet is derived from.
type SheetInput struct {
	Class       string
	Level       int
	RulesetYear int
	HitDie      int
	Scores      map[Ability]int
	SaveProfs   []Ability
	SkillProfs  []Skill
	Armor       *Armor
	ShieldBonus int
	SpeedFeet   int
}

// Sheet is what the character sheet shows.
type Sheet struct {
	ProficiencyBonus  int
	ArmorClass        int
	Initiative        int
	SpeedFeet         int
	PassivePerception int
	Saves             []Save
	Skills            []SkillBonus
	Resources         []Resource
	Warnings          []string
}

// BuildSheet derives the sheet values.
func BuildSheet(in SheetInput) Sheet {
	pb := ProficiencyBonus(in.Level)
	mod := func(a Ability) int { return Modifier(in.Scores[a]) }
	s := Sheet{
		ProficiencyBonus:  pb,
		ArmorClass:        ArmorClass(mod(Dexterity), in.Armor, in.ShieldBonus),
		Initiative:        mod(Dexterity),
		SpeedFeet:         in.SpeedFeet,
		PassivePerception: 0,
		Saves:             make([]Save, 0, 6),
		Skills:            make([]SkillBonus, 0, 18),
		Resources:         FirstLevelResources(in.Class, in.HitDie, in.RulesetYear),
		Warnings:          []string{},
	}
	for _, a := range Abilities() {
		prof := contains(in.SaveProfs, a)
		s.Saves = append(s.Saves, Save{Ability: a, Score: in.Scores[a], Modifier: mod(a), Proficient: prof, Bonus: mod(a) + pbIf(prof, pb)})
	}
	for _, sk := range Skills() {
		prof := contains(in.SkillProfs, sk.Skill)
		b := SkillBonus{Skill: sk.Skill, Ability: sk.Ability, Proficient: prof, Bonus: mod(sk.Ability) + pbIf(prof, pb)}
		if sk.Skill == "perception" {
			s.PassivePerception = 10 + b.Bonus
		}
		s.Skills = append(s.Skills, b)
	}
	if in.Armor != nil && in.Armor.StrengthRequired > in.Scores[Strength] {
		s.SpeedFeet -= 10
		s.Warnings = append(s.Warnings, "Your armour is too heavy: speed drops by 10 feet.")
	}
	if in.Armor != nil && in.Armor.Stealth {
		s.Warnings = append(s.Warnings, "Your armour gives Disadvantage on Stealth checks.")
	}
	return s
}

func pbIf(ok bool, pb int) int {
	if ok {
		return pb
	}
	return 0
}
