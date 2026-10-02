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
		if armor.DexCap >= 0 {
			dex = min(dex, armor.DexCap)
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

// fullCasterSlots are a full caster's spell slots per spell level, by character level (SRD 5.2).
var fullCasterSlots = [20][9]int{ //nolint:gochecknoglobals // a fixed table
	{2},
	{3},
	{4, 2},
	{4, 3},
	{4, 3, 2},
	{4, 3, 3},
	{4, 3, 3, 1},
	{4, 3, 3, 2},
	{4, 3, 3, 3, 1},
	{4, 3, 3, 3, 2},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 2, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 1, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 1, 1, 1},
	{4, 3, 3, 3, 3, 2, 2, 1, 1},
}

// slotsAt are the spell slots per spell level a class has at a level: half casters (who cast from
// level 1 in SRD 5.2) as a full caster of half their level rounded up, pact casters a few slots of
// one level.
func slotsAt(class string, level int) [9]int {
	level = min(max(level, 1), 20)
	switch CasterFor(class) {
	case FullCaster:
		return fullCasterSlots[level-1]
	case HalfCaster:
		return fullCasterSlots[(level+1)/2-1]
	case PactCaster:
		var out [9]int
		out[min((level+1)/2, 5)-1] = pactSlots(level)
		return out
	case NoCaster:
	}
	return [9]int{}
}

// pactSlots is how many slots a pact caster has at a level.
func pactSlots(level int) int {
	switch {
	case level >= 17:
		return 4
	case level >= 11:
		return 3
	case level >= 2:
		return 2
	default:
		return 1
	}
}

// ResourcesAt are the pools a new character has at a level: one Hit Die per level and the spell slots
// of its class.
func ResourcesAt(class string, hitDie, level int) []Resource {
	level = max(level, 1)
	out := []Resource{{Key: "hit-dice", Label: "Hit Dice (d" + strconv.Itoa(hitDie) + ")", Current: level, Max: level}}
	for i, n := range slotsAt(class, level) {
		if n > 0 {
			spell := strconv.Itoa(i + 1)
			out = append(out, Resource{Key: "spell-slots-" + spell, Label: "Level " + spell + " spell slots", Current: n, Max: n})
		}
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
	Expertise  bool
	Bonus      int
}

// SheetInput is everything the sheet is derived from.
type SheetInput struct {
	Class      string
	Level      int
	HitDie     int
	Scores     map[Ability]int
	SaveProfs  []Ability
	SkillProfs []Skill
	// Expertise doubles the proficiency bonus for skills the character is proficient in.
	Expertise   []Skill
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
		Resources:         ResourcesAt(in.Class, in.HitDie, in.Level),
		Warnings:          []string{},
	}
	for _, a := range Abilities() {
		prof := contains(in.SaveProfs, a)
		s.Saves = append(s.Saves, Save{Ability: a, Score: in.Scores[a], Modifier: mod(a), Proficient: prof, Bonus: mod(a) + pbIf(prof, pb)})
	}
	for _, sk := range Skills() {
		prof := contains(in.SkillProfs, sk.Skill)
		expert := prof && contains(in.Expertise, sk.Skill)
		b := SkillBonus{Skill: sk.Skill, Ability: sk.Ability, Proficient: prof, Expertise: expert, Bonus: mod(sk.Ability) + pbIf(prof, pb) + pbIf(expert, pb)}
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
