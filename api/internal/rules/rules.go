// Package rules is the pure D&D 5e rules engine: no I/O, no clock, no randomness of its own.
package rules

import "fmt"

// ViolationError is a rules check that failed, with a reason fit to show the player.
type ViolationError struct {
	Reason string
}

func (v *ViolationError) Error() string {
	return "rules: " + v.Reason
}

func violation(format string, args ...any) error {
	return &ViolationError{Reason: fmt.Sprintf(format, args...)}
}

// Ability is one of the six ability scores.
type Ability string

// The six abilities, in sheet order.
const (
	Strength     Ability = "strength"
	Dexterity    Ability = "dexterity"
	Constitution Ability = "constitution"
	Intelligence Ability = "intelligence"
	Wisdom       Ability = "wisdom"
	Charisma     Ability = "charisma"
)

// Abilities lists the six abilities in sheet order.
func Abilities() []Ability {
	return []Ability{Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma}
}

// Valid reports whether a is one of the six abilities.
func (a Ability) Valid() bool {
	for _, x := range Abilities() {
		if x == a {
			return true
		}
	}
	return false
}

// Modifier is the ability modifier for a score.
func Modifier(score int) int {
	return (score+10)/2 - 10
}

// ProficiencyBonus is the proficiency bonus at a character level (1..20).
func ProficiencyBonus(level int) int {
	return 2 + (level-1)/4
}

// ProficiencyByChallenge is a monster's proficiency bonus by Challenge Rating: +2 up to 4, one more for
// every four ratings after.
func ProficiencyByChallenge(cr float64) int {
	if cr < 5 {
		return 2
	}
	return 2 + (int(cr)-1)/4
}
