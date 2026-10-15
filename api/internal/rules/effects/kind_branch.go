package effects

import (
	"strconv"
	"strings"
)

// Branch adds Parts when its condition holds at the moment the Effect resolves against a creature
// ("if the save fails by 5 or more").
type Branch struct {
	When Condition
	Then []Part
}

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

// Text writes a Branch's condition as the start of a sentence.
func (c Condition) Text() string {
	switch c.Kind {
	case FirstEachTurn:
		return "The first time on a turn the effect touches a creature"
	case CreatureIs:
		return "If the creature is " + article(c.Type) + " " + c.Type
	case FailsBy, HPAtMost:
	}
	return "If a creature " + c.clause()
}

// clause is the condition as what a creature does or is, to join with others: "fails the save".
func (c Condition) clause() string {
	switch c.Kind {
	case FailsBy:
		if c.N == 1 {
			return "fails the save"
		}
		return "fails the save by " + strconv.Itoa(c.N) + " or more"
	case HPAtMost:
		return "has " + strconv.Itoa(c.N) + " Hit Points or fewer"
	case FirstEachTurn:
		return "is touched by the effect for the first time on a turn"
	case CreatureIs:
	}
	return "is " + article(c.Type) + " " + c.Type
}

func (Branch) isPart() {}

// Kind is branch.
func (Branch) Kind() string { return "branch" }

// Text is the condition, then the Parts it adds: branches nested alone join their tests, and Parts that
// say what happens to the creature picked out read as one clause.
func (b Branch) Text() string {
	lead, then := b.When.Text(), b.Then
	clauses := []string{b.When.clause()}
	for inner, ok := b.lone(); ok; inner, ok = inner.lone() {
		clauses = append(clauses, inner.When.clause())
		then = inner.Then
	}
	if len(clauses) > 1 {
		lead = "If a creature " + strings.Join(clauses, " and ")
	}
	if done, ok := consequences(then); ok {
		return lead + ", it " + strings.Join(done, " and ") + "."
	}
	return lead + ", " + lower(strings.Join(sentences(then), " "))
}

// lone is the branch beneath, when it is all this branch adds.
func (b Branch) lone() (Branch, bool) {
	if len(b.Then) != 1 {
		return b, false
	}
	inner, ok := b.Then[0].(Branch)
	return inner, ok
}

// consequences is what each Part does to the creature a branch picked out, when every one can say so.
func consequences(ps []Part) ([]string, bool) {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		c, ok := p.(Consequent)
		if !ok {
			return nil, false
		}
		out = append(out, c.Consequence())
	}
	return out, len(out) > 0
}

// Groups holds the Parts the branch adds under then.
func (b Branch) Groups() []Group {
	return []Group{{Name: "then", Parts: b.Then}}
}

// Landed is the branch's Parts when its condition holds for the creature, and none otherwise.
func (b Branch) Landed(s Situation) []Part {
	if b.When.Holds(s) {
		return b.Then
	}
	return nil
}

func conditionKinds() []string {
	return []string{string(FailsBy), string(HPAtMost), string(FirstEachTurn), string(CreatureIs)}
}

type branchKind struct{}

func (branchKind) Describe() Description {
	return Description{
		Slug: "branch", Label: "Branch", Summary: "Adds the Parts beneath it for a creature when its test holds as the Effect lands.",
		Fields: []Field{
			{Name: "when", Label: "When", Type: ChoiceField, Options: conditionKinds(), Required: true, Min: 0, Max: 0},
			{Name: "n", Label: "By or at most", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 1000},
			{Name: "creature_type", Label: "Creature type", Type: TextField, Options: nil, Required: false, Min: 0, Max: 0},
		},
		Groups: []string{"then"}, OpenGroups: false,
	}
}

func (branchKind) Hints() Hints {
	return Hints{Order: nil, Sections: []Section{{Label: "Test", Fields: []string{"when", "n", "creature_type"}}}, Help: []Help{
		{Field: "n", Text: "For fails by, how far the save fails; for HP at most, the Hit Points."},
		{Field: "creature_type", Text: "For creature is, such as fey."},
	}}
}

func (branchKind) Decode(v Values, beneath Groups) (Part, error) {
	b := Branch{When: Condition{Kind: ConditionKind(v["when"].Text), N: v["n"].Number, Type: v["creature_type"].Text}, Then: beneath.Of("then")}
	if err := oneOf("when", string(b.When.Kind), conditionKinds()); err != nil {
		return nil, err
	}
	if b.When.N < 0 || b.When.N > 1000 {
		return nil, refuse("n", "runs from 0 to 1000")
	}
	if len(b.When.Type) > 40 || (b.When.Kind == CreatureIs && b.When.Type == "") {
		return nil, refuse("creature_type", "name the creature type in up to 40 letters")
	}
	if len(b.Then) == 0 {
		return nil, refuse("then", "add at least one Part beneath the branch")
	}
	return b, nil
}
