package effects

import (
	"strconv"
	"strings"
)

// Choice offers modes; whoever applies the Effect picks one, and only that mode's components count
// (Enlarge/Reduce).
type Choice struct {
	Modes []Mode
}

// Mode is one option of a Choice.
type Mode struct {
	Name       string
	Components []Component
}

// Branch adds components when its condition holds at the moment the Effect resolves against a
// creature ("if the save fails by 5 or more").
type Branch struct {
	When Condition
	Then []Component
}

func (Choice) isComponent() {}
func (Branch) isComponent() {}

// ConditionKind is what a Branch tests.
type ConditionKind string

// Condition kinds.
const (
	// FailsBy holds when the creature failed its save by at least N.
	FailsBy ConditionKind = "fails_by"
	// HPAtMost holds when the creature has N hit points or fewer.
	HPAtMost ConditionKind = "hp_at_most"
	// FirstEachTurn holds the first time the Effect touches the creature on a turn.
	FirstEachTurn ConditionKind = "first_each_turn"
	// CreatureIs holds for creatures of one type.
	CreatureIs ConditionKind = "creature_is"
)

// Condition is a Branch's test.
type Condition struct {
	Kind ConditionKind
	N    int
	Type string
}

// Situation is what a Branch is tested against: how a save went, the creature's hit points and type,
// and whether this is the first time on the turn.
type Situation struct {
	Saved bool
	// Margin is the DC minus the save's total: how far a failed save fell short.
	Margin int
	HP     int
	First  bool
	Type   string
}

// Holds reports whether a condition is true in a situation.
func (c Condition) Holds(s Situation) bool {
	switch c.Kind {
	case FailsBy:
		return !s.Saved && s.Margin >= c.N
	case HPAtMost:
		return s.HP <= c.N
	case FirstEachTurn:
		return s.First
	case CreatureIs:
		return strings.EqualFold(s.Type, c.Type)
	}
	return false
}

// Parts are the components that count for an Effect applied in a mode: its own, with each Choice
// replaced by the chosen mode's. Branches count only when tested.
func (d Definition) Parts(mode string) []Component {
	return parts(d.Components, mode)
}

func parts(cs []Component, mode string) []Component {
	var out []Component
	for _, c := range cs {
		switch c := c.(type) {
		case Choice:
			for _, m := range c.Modes {
				if m.Name == mode {
					out = append(out, parts(m.Components, mode)...)
				}
			}
		case Branch:
		default:
			out = append(out, c)
		}
	}
	return out
}

// Modes lists the names of an Effect's modes, empty when it offers no choice.
func (d Definition) Modes() []string {
	var out []string
	for _, c := range d.Components {
		if ch, ok := c.(Choice); ok {
			for _, m := range ch.Modes {
				out = append(out, m.Name)
			}
		}
	}
	return out
}

// Branches lists the components an Effect adds for a creature in a situation, in order.
func (d Definition) Branches(mode string, s Situation) []Component {
	return branches(d.Components, mode, s)
}

func branches(cs []Component, mode string, s Situation) []Component {
	var out []Component
	for _, c := range cs {
		switch c := c.(type) {
		case Branch:
			if c.When.Holds(s) {
				out = append(out, parts(c.Then, mode)...)
				out = append(out, branches(c.Then, mode, s)...)
			}
		case Choice:
			for _, m := range c.Modes {
				if m.Name == mode {
					out = append(out, branches(m.Components, mode, s)...)
				}
			}
		default:
		}
	}
	return out
}

// walk calls visit on every component of an Effect, in modes and branches too.
func walk(cs []Component, visit func(Component)) {
	for _, c := range cs {
		visit(c)
		switch c := c.(type) {
		case Choice:
			for _, m := range c.Modes {
				walk(m.Components, visit)
			}
		case Branch:
			walk(c.Then, visit)
		default:
		}
	}
}

// DurationKind is how long an Effect lasts.
type DurationKind string

// Duration kinds.
const (
	Instant        DurationKind = "instant"
	Rounds         DurationKind = "rounds"
	Minutes        DurationKind = "minutes"
	Hours          DurationKind = "hours"
	UntilDispelled DurationKind = "until_dispelled"
	UntilRest      DurationKind = "until_rest"
	Permanent      DurationKind = "permanent"
	EndOfNextTurn  DurationKind = "end_of_next_turn"
	// UntilCured is a lingering injury: it lasts until its own cure is applied.
	UntilCured DurationKind = "until_cured"
)

// Duration is how long an Effect lasts, and the save its bearer repeats to end it early. The zero
// Duration leaves the length to whoever applies the Effect.
type Duration struct {
	Kind   DurationKind
	Amount int
	// RepeatSave is the ability the bearer saves with to end the Effect, at the end of each of its turns.
	RepeatSave string
}

// RoundsPerMinute is how many six-second rounds make a minute.
const RoundsPerMinute = 10

// Rounds is how many rounds an Effect lasts; zero when its end is not counted in rounds.
func (d Duration) Rounds() int {
	switch d.Kind {
	case Rounds:
		return d.Amount
	case Minutes:
		return d.Amount * RoundsPerMinute
	case Hours:
		return d.Amount * RoundsPerMinute * 60
	case EndOfNextTurn:
		return 1
	case Instant, UntilDispelled, UntilRest, Permanent, UntilCured:
	}
	return 0
}

// EndsOnRest reports whether finishing a rest ends the Effect.
func (d Duration) EndsOnRest() bool {
	return d.Kind == UntilRest
}

// Lingers reports whether the Effect lasts until its own cure: no rest ends it, and it goes with its
// bearer from one Session to the next.
func (d Duration) Lingers() bool {
	return d.Kind == UntilCured
}

// Axis is what an Effect scales with.
type Axis string

// Scaling axes.
const (
	SlotLevel      Axis = "slot_level"
	CharacterLevel Axis = "character_level"
	ClassLevel     Axis = "class_level"
	TableColumn    Axis = "table_column"
)

// Step is the dice an Effect deals from a level on.
type Step struct {
	At   int
	Dice string
}

// Scaling grows an Effect's dice: by Dice for each slot level above Base, to a Step's dice at a
// character or class level, or to the value of a class table's column.
type Scaling struct {
	Axis   Axis
	Class  string
	Column string
	Base   int
	Dice   string
	Steps  []Step
}

// Level is what a creature casts or acts at: the slot used, its character level, its level in each
// class, and its class tables' values.
type Level struct {
	Slot      int
	Character int
	Classes   map[string]int
	Columns   map[string]string
}

// Apply scales dice to a level.
func (s Scaling) Apply(dice string, at Level) string {
	switch s.Axis {
	case SlotLevel:
		for range at.Slot - s.Base {
			dice = addDice(dice, s.Dice)
		}
	case CharacterLevel:
		dice = stepped(dice, s.Steps, at.Character)
	case ClassLevel:
		dice = stepped(dice, s.Steps, at.Classes[s.Class])
	case TableColumn:
		if v := at.Columns[s.Column]; v != "" {
			dice = v
		}
	}
	return dice
}

func stepped(dice string, steps []Step, level int) string {
	for _, st := range steps {
		if level >= st.At {
			dice = st.Dice
		}
	}
	return dice
}

// addDice adds more dice to notation, folding dice of the same size: 8d6 and 1d6 make 9d6.
func addDice(dice, more string) string {
	n, faces, ok := strings.Cut(dice, "d")
	m, moreFaces, okMore := strings.Cut(more, "d")
	a, errA := strconv.Atoi(n)
	b, errB := strconv.Atoi(m)
	if !ok || !okMore || faces != moreFaces || errA != nil || errB != nil {
		return dice + "+" + more
	}
	return strconv.Itoa(a+b) + "d" + faces
}
