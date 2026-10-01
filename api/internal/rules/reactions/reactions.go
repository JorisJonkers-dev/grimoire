// Package reactions decides, from a Controller's reaction settings, whether a reaction is offered at
// all, and whether it is taken without asking.
package reactions

// Mode is how a Controller wants one kind of reaction handled.
type Mode string

// Modes.
const (
	Ask    Mode = "ask"
	Always Mode = "always"
	Never  Mode = "never"
)

// Condition narrows Always: the reaction is taken without asking only when it holds; otherwise it asks.
type Condition string

// Conditions.
const (
	Anytime Condition = ""
	// TargetBloodied holds when the creature the reaction is against is at half its hit points or fewer.
	TargetBloodied Condition = "target_bloodied"
)

// Setting is a Controller's choice for one kind of reaction.
type Setting struct {
	Mode      Mode
	Condition Condition
}

// Situation is what a condition reads: the hit points of the creature the reaction is against.
type Situation struct {
	TargetHP    int
	TargetHPMax int
}

// Decision is what to do with a reaction that could be taken.
type Decision struct {
	Offer bool
	Auto  bool
}

// Decide applies a setting to a situation: Never offers nothing, Always takes the reaction when its
// condition holds and asks otherwise, and Ask (or no setting) asks.
func Decide(s Setting, at Situation) Decision {
	switch s.Mode {
	case Never:
		return Decision{Offer: false, Auto: false}
	case Always:
		return Decision{Offer: true, Auto: holds(s.Condition, at)}
	case Ask:
	}
	return Decision{Offer: true, Auto: false}
}

func holds(c Condition, at Situation) bool {
	switch c {
	case TargetBloodied:
		return at.TargetHPMax > 0 && at.TargetHP*2 <= at.TargetHPMax
	case Anytime:
		return true
	}
	return false
}

// Valid reports whether a mode and condition are ones the settings know.
func Valid(s Setting) bool {
	okMode := s.Mode == Ask || s.Mode == Always || s.Mode == Never
	okCondition := s.Condition == Anytime || s.Condition == TargetBloodied
	return okMode && okCondition
}
