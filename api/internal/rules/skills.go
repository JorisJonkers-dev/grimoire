package rules

// Skill is one of the eighteen skills.
type Skill string

// SkillAbility pairs a skill with the ability it uses.
type SkillAbility struct {
	Skill   Skill
	Ability Ability
}

// Skills lists every skill with its ability, alphabetically.
func Skills() []SkillAbility {
	return []SkillAbility{
		{Skill: "acrobatics", Ability: Dexterity},
		{Skill: "animal-handling", Ability: Wisdom},
		{Skill: "arcana", Ability: Intelligence},
		{Skill: "athletics", Ability: Strength},
		{Skill: "deception", Ability: Charisma},
		{Skill: "history", Ability: Intelligence},
		{Skill: "insight", Ability: Wisdom},
		{Skill: "intimidation", Ability: Charisma},
		{Skill: "investigation", Ability: Intelligence},
		{Skill: "medicine", Ability: Wisdom},
		{Skill: "nature", Ability: Intelligence},
		{Skill: "perception", Ability: Wisdom},
		{Skill: "performance", Ability: Charisma},
		{Skill: "persuasion", Ability: Charisma},
		{Skill: "religion", Ability: Intelligence},
		{Skill: "sleight-of-hand", Ability: Dexterity},
		{Skill: "stealth", Ability: Dexterity},
		{Skill: "survival", Ability: Wisdom},
	}
}

// Valid reports whether s is one of the eighteen skills.
func (s Skill) Valid() bool {
	for _, x := range Skills() {
		if x.Skill == s {
			return true
		}
	}
	return false
}

// ClassSkillCount is how many skills a class lets a first-level character choose.
func ClassSkillCount(class string) int {
	switch class {
	case "rogue":
		return 4
	case "bard", "ranger":
		return 3
	default:
		return 2
	}
}

// ValidateSkills checks the class skill choices: the right count, real skills, no repeats, and none
// the background already grants.
func ValidateSkills(class string, chosen, fromBackground []Skill) error {
	if want := ClassSkillCount(class); len(chosen) != want {
		return violation("choose %d class skills", want)
	}
	seen := map[Skill]bool{}
	for _, s := range chosen {
		switch {
		case !s.Valid():
			return violation("%q is not a skill", s)
		case seen[s]:
			return violation("%s is chosen twice", s)
		case contains(fromBackground, s):
			return violation("your background already grants %s", s)
		}
		seen[s] = true
	}
	return nil
}
