package effects

import (
	"regexp"
	"strconv"
	"strings"
)

// Axis is what Scaling grows with.
type Axis string

// Scaling axes.
const (
	SlotLevel      Axis = "slot_level"
	CharacterLevel Axis = "character_level"
	ClassLevel     Axis = "class_level"
	TableColumn    Axis = "table_column"
	Charges        Axis = "charges"
)

// Axes lists every axis, in the order the builder offers them.
func Axes() []string {
	return []string{string(SlotLevel), string(CharacterLevel), string(ClassLevel), string(TableColumn), string(Charges)}
}

// OnActivation is the Part a Scaling names when it grows one of the Activation's own values.
const OnActivation = -1

// Scaling grows one scalable value of an Effect with the level it is activated at. Part counts the
// Effect's Parts depth first from 0, as they are stored; Class is the class a class level or a class
// table belongs to.
type Scaling struct {
	Part  int
	Field string
	Axis  Axis
	Class string
	Rule  Rule
}

// Rule is how a scalable value grows.
//
//sumtype:decl
type Rule interface{ isRule() }

// Linear adds Per for each level above Base.
type Linear struct {
	Base int
	Per  Value
}

// Table sets the value of the last step at or below the level; below the first, the value stays.
type Table struct {
	Steps []Step
}

// Formula works the value out from the level.
type Formula struct {
	Expr string
}

// Column takes the value of a column of the class's table.
type Column struct {
	Name string
}

func (Linear) isRule()  {}
func (Table) isRule()   {}
func (Formula) isRule() {}
func (Column) isRule()  {}

// Step is the value from a level on.
type Step struct {
	At    int
	Value Value
}

// Level is what an Effect is activated at: the slot used, the character level, the level in each
// class, the class tables' values, and the Charges spent. Zero is the Effect as written.
type Level struct {
	Slot      int
	Character int
	Classes   map[string]int
	Columns   map[string]string
	Charges   int
}

// Scalable is a field that grows with the level its Effect is activated at; Formula lets it follow a
// formula of that level.
type Scalable struct {
	Field   string
	Formula bool
}

// Scaler is a kind with fields that scale.
type Scaler interface {
	Scales() []Scalable
}

// ScalesOf is the fields a kind lets scale; none when it marks none.
func ScalesOf(k Kind) []Scalable {
	if s, ok := k.(Scaler); ok {
		return s.Scales()
	}
	return nil
}

// Grower is a Part whose scalable fields grow.
type Grower interface {
	Part
	// Grow is the Part with one of its scalable fields grown by by; any other field leaves it as it is.
	Grow(field string, by func(Value) Value) Part
}

// ActivationFields are the Activation's own values Scaling can grow: how long the Effect lasts, in what
// unit, whether it needs concentration, and how many targets it takes.
func ActivationFields() []Field {
	return []Field{
		{Name: "duration", Label: "Duration", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 1000},
		{Name: "duration_unit", Label: "Duration unit", Type: ChoiceField, Options: durationKinds(), Required: false, Min: 0, Max: 0},
		{Name: "concentration", Label: "Concentration", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
		{Name: "targets", Label: "Number of targets", Type: NumberField, Options: nil, Required: false, Min: 1, Max: 50},
	}
}

// ActivationScales marks every Activation field as scalable; a duration may follow a formula.
func ActivationScales() []Scalable {
	return []Scalable{{Field: "duration", Formula: true}, {Field: "duration_unit", Formula: false}, {Field: "concentration", Formula: false}, {Field: "targets", Formula: true}}
}

func durationKinds() []string {
	return []string{
		string(Instant), string(Rounds), string(Minutes), string(Hours), string(UntilDispelled), string(UntilRest), string(Permanent),
		string(EndOfNextTurn), string(UntilCured),
	}
}

// LowestSlot is the lowest slot an Effect that scales by slot is activated with: the least base of its
// linear slot rules, else 1. ok is false when nothing in it scales by slot.
func (d Definition) LowestSlot() (int, bool) {
	lowest, ok := 0, false
	for _, s := range d.Scaling {
		if s.Axis != SlotLevel {
			continue
		}
		base := 1
		if r, linear := s.Rule.(Linear); linear {
			base = r.Base
		}
		if ok {
			base = min(base, lowest)
		}
		lowest, ok = base, true
	}
	return lowest, ok
}

// At is the Effect as it is activated at a level, with every Scaling applied in order.
func (d Definition) At(l Level) Definition {
	for _, s := range d.Scaling {
		if s.Part == OnActivation {
			d = d.activated(s, l)
			continue
		}
		next := 0
		d.Parts = grow(d.Parts, s, l, &next)
	}
	return d
}

// grow applies a Scaling to the Part it names among Parts counted depth first, and rebuilds the Parts
// that hold others.
func grow(ps []Part, s Scaling, l Level, next *int) []Part {
	var out []Part
	for _, p := range ps {
		at := *next
		*next++
		if g, ok := p.(Grower); ok && at == s.Part {
			p = g.Grow(s.Field, func(v Value) Value { return s.grow(v, l) })
		}
		if g, ok := p.(Grouper); ok {
			groups := make([]Group, 0, len(g.Groups()))
			for _, group := range g.Groups() {
				groups = append(groups, Group{Name: group.Name, Parts: grow(group.Parts, s, l, next)})
			}
			p = g.Regroup(groups)
		}
		out = append(out, p)
	}
	return out
}

// activated applies a Scaling to one of the Activation's own values.
func (d Definition) activated(s Scaling, l Level) Definition {
	switch s.Field {
	case "duration":
		d.Duration.Amount = s.grow(numberValue(d.Duration.Amount), l).Number
	case "duration_unit":
		d.Duration.Kind = DurationKind(s.grow(textValue(string(d.Duration.Kind)), l).Text)
	case "concentration":
		d.Concentration = s.grow(Value{Text: "", Number: 0, Flag: d.Concentration, Texts: nil}, l).Flag
	case "targets":
		d.Targeting.Count = s.grow(numberValue(d.Targeting.Count), l).Number
	}
	return d
}

// grow is a value at a level, by the Scaling's rule.
func (s Scaling) grow(v Value, l Level) Value {
	n := s.levelOf(l)
	switch r := s.Rule.(type) {
	case Linear:
		for range n - r.Base {
			v = Value{Text: plusDice(v.Text, r.Per.Text), Number: v.Number + r.Per.Number, Flag: v.Flag, Texts: v.Texts}
		}
	case Table:
		for _, st := range r.Steps {
			if n >= st.At {
				v = st.Value
			}
		}
	case Formula:
		if x, ok := Evaluate(r.Expr, n); ok && n > 0 {
			v.Number = x
		}
	case Column:
		v = column(v, l.Columns[r.Name])
	}
	return v
}

// column is a value as a class table's column gives it: its text, and its number when it is one; an
// empty column leaves the value.
func column(v Value, c string) Value {
	if c == "" {
		return v
	}
	v.Text = c
	if x, err := strconv.Atoi(c); err == nil {
		v.Number = x
	}
	return v
}

func numberValue(n int) Value {
	return Value{Text: "", Number: n, Flag: false, Texts: nil}
}

func textValue(s string) Value {
	return Value{Text: s, Number: 0, Flag: false, Texts: nil}
}

// levelOf is the level a Scaling's axis is activated at; a table column has none of its own.
func (s Scaling) levelOf(l Level) int {
	switch s.Axis {
	case SlotLevel:
		return l.Slot
	case CharacterLevel:
		return l.Character
	case ClassLevel:
		return l.Classes[s.Class]
	case Charges:
		return l.Charges
	case TableColumn:
	}
	return 0
}

// plusDice adds dice to dice, folding dice of the same size; with no dice to add, it adds none.
func plusDice(dice, more string) string {
	if more == "" {
		return dice
	}
	return addDice(dice, more)
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

// Evaluate works out a formula of the level: whole numbers of up to four digits, level, +, -, * and
// brackets; ok is false for anything else.
func Evaluate(expr string, level int) (int, bool) {
	toks := formulaToken.FindAllString(expr, -1)
	if strings.Join(toks, "") != strings.Join(strings.Fields(expr), "") {
		return 0, false
	}
	f := formula{toks: toks, level: level}
	v, ok := f.sum()
	return v, ok && len(f.toks) == 0
}

var formulaToken = regexp.MustCompile(`level|[0-9]+|[-+*()]`)

// formula reads tokens by precedence, sums of products of factors, taking each token once.
type formula struct {
	toks  []string
	level int
}

func (f *formula) peek() string {
	if len(f.toks) == 0 {
		return ""
	}
	return f.toks[0]
}

func (f *formula) take() string {
	t := f.peek()
	if len(f.toks) > 0 {
		f.toks = f.toks[1:]
	}
	return t
}

func (f *formula) sum() (int, bool) {
	v, ok := f.product()
	for ok && (f.peek() == "+" || f.peek() == "-") {
		minus := f.take() == "-"
		var w int
		w, ok = f.product()
		if minus {
			w = -w
		}
		v += w
	}
	return v, ok
}

func (f *formula) product() (int, bool) {
	v, ok := f.factor()
	for ok && f.peek() == "*" {
		f.take()
		var w int
		w, ok = f.factor()
		v *= w
	}
	return v, ok
}

func (f *formula) factor() (int, bool) {
	t := f.take()
	switch t {
	case "level":
		return f.level, true
	case "(":
		v, ok := f.sum()
		return v, ok && f.take() == ")"
	}
	n, err := strconv.Atoi(t)
	return n, err == nil && len(t) <= 4
}
