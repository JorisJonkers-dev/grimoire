package rules

// TakeDamage applies damage to hit points and temporary hit points: the temporary ones soak it first,
// and hit points stop at 0.
func TakeDamage(hp, temp, amount int) (int, int) {
	amount = max(amount, 0)
	soaked := min(temp, amount)
	return max(hp-(amount-soaked), 0), temp - soaked
}

// Heal restores hit points up to the maximum.
func Heal(hp, maxHP, amount int) int {
	return min(hp+max(amount, 0), maxHP)
}

// GainTempHP grants temporary hit points; they do not stack, so the higher amount stays.
func GainTempHP(temp, amount int) int {
	return max(temp, amount)
}

// Proficiencies are the armour and weapons a class trains a character in (SRD 5.2).
type Proficiencies struct {
	Armor   []string
	Weapons []string
}

// ClassProficiencies are a class's armour and weapon training; a class Grimoire does not know has none.
func ClassProficiencies(class string) Proficiencies {
	simple, martial := "Simple weapons", "Martial weapons"
	switch class {
	case "barbarian":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Shields"}, Weapons: []string{simple, martial}}
	case "fighter", "paladin":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Heavy armor", "Shields"}, Weapons: []string{simple, martial}}
	case "ranger":
		return Proficiencies{Armor: []string{"Light armor", "Medium armor", "Shields"}, Weapons: []string{simple, martial}}
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
