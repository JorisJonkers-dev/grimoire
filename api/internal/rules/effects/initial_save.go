package effects

import "strconv"

// InitialSave is the saving throw each target makes as the Effect is activated; the Effect lands only on
// those who fail. A DC of 0 is the caster's spell save DC; the zero InitialSave asks for none.
type InitialSave struct {
	Ability string
	DC      int
}

// Check refuses an initial save the rules cannot hold: an unknown ability, a DC with no ability, or a DC
// out of range.
func (s InitialSave) Check() error {
	switch {
	case s.Ability == "" && s.DC != 0:
		return refuse("ability", "name the ability")
	case s.Ability != "" && oneOf("ability", s.Ability, abilities()) != nil:
		return refuse("ability", "choose one of the options")
	case s.DC < 0 || s.DC > 30:
		return refuse("dc", "runs from 1 to 30")
	}
	return nil
}

// Text is the initial save as a spell prints it; empty when there is none.
func (s InitialSave) Text() string {
	switch {
	case s.Ability == "":
		return ""
	case s.DC == 0:
		return "Each target makes a " + title(s.Ability) + " saving throw against your spell save DC; the effect lands on a failure."
	}
	return "Each target makes a DC " + strconv.Itoa(s.DC) + " " + title(s.Ability) + " saving throw; the effect lands on a failure."
}
