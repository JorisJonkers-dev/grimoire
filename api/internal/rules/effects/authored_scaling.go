package effects

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// AuthoredScaling is a Scaling as its author writes it, on the Part whose field grows or on the
// Activation: the field, the axis, and the rule with its own values.
type AuthoredScaling struct {
	Field   string         `json:"field"`
	Axis    string         `json:"axis"`
	Class   string         `json:"class,omitempty"`
	Rule    string         `json:"rule"`
	Base    int            `json:"base,omitempty"`
	Per     *Value         `json:"per,omitempty"`
	Steps   []AuthoredStep `json:"steps,omitempty"`
	Formula string         `json:"formula,omitempty"`
	Column  string         `json:"column,omitempty"`
}

// AuthoredStep is one step of a table: the value from a level on.
type AuthoredStep struct {
	At    int   `json:"at"`
	Value Value `json:"value"`
}

// maxScaling is how many rules may grow one Part, or the Activation.
const maxScaling = 10

// DecodeScaling checks the Scaling written on authored Parts against each Part's kind and makes it,
// counting the Parts depth first from first, the place the Effect's own Parts begin.
func DecodeScaling(in []Authored, first int) ([]Scaling, error) {
	next := first
	return scalingOf(in, &next)
}

func scalingOf(in []Authored, next *int) ([]Scaling, error) {
	var out []Scaling
	for i, a := range in {
		at := *next
		*next++
		mine, err := a.scaling(at)
		if err != nil {
			return nil, PartError{At: "part " + strconv.Itoa(i+1), Err: err}
		}
		out = append(out, mine...)
		for _, g := range a.Groups {
			beneath, err := scalingOf(g.Parts, next)
			if err != nil {
				return nil, PartError{At: "part " + strconv.Itoa(i+1), Err: PartError{At: g.Name, Err: err}}
			}
			out = append(out, beneath...)
		}
	}
	return out, nil
}

// scaling decodes the Scaling written on one Part, the Part at.
func (a Authored) scaling(at int) ([]Scaling, error) {
	if len(a.Scaling) == 0 {
		return nil, nil
	}
	k, ok := KindOf(a.Kind)
	if !ok {
		return nil, refuse("kind", "choose a kind the rules know")
	}
	return DecodePlaced(a.Scaling, []Place{{Part: at, Fields: k.Describe().Fields, Scales: ScalesOf(k)}}, "a Part")
}

// Place is where fields that scale sit: on a Part, or on the Activation.
type Place struct {
	Part   int
	Fields []Field
	Scales []Scalable
}

// ActivationPlace is the Activation's own fields.
func ActivationPlace() Place {
	return Place{Part: OnActivation, Fields: ActivationFields(), Scales: ActivationScales()}
}

// DecodeActivationScaling checks the Scaling written on the Activation's own values and makes it.
func DecodeActivationScaling(in []AuthoredScaling) ([]Scaling, error) {
	return DecodePlaced(in, []Place{ActivationPlace()}, "the Activation")
}

// DecodePlaced checks Scaling written on what holds several places, each rule over the field of the
// first place that lets it scale, and makes it; what names the whole in a refusal.
func DecodePlaced(in []AuthoredScaling, places []Place, what string) ([]Scaling, error) {
	if len(in) > maxScaling {
		return nil, refuse("scaling", "scale "+what+" by at most "+strconv.Itoa(maxScaling)+" rules")
	}
	var out []Scaling
	for i, a := range in {
		s, err := a.decode(places)
		if err != nil {
			return nil, PartError{At: "scaling " + strconv.Itoa(i+1), Err: err}
		}
		out = append(out, s)
	}
	return out, nil
}

// decode checks one rule against the field it grows.
func (a AuthoredScaling) decode(places []Place) (Scaling, error) {
	for _, p := range places {
		si := slices.IndexFunc(p.Scales, func(s Scalable) bool { return s.Field == a.Field })
		fi := slices.IndexFunc(p.Fields, func(f Field) bool { return f.Name == a.Field })
		if si >= 0 && fi >= 0 {
			return a.decodeOn(p.Part, p.Fields[fi], p.Scales[si])
		}
	}
	return Scaling{}, refuse("field", "choose a field that scales")
}

// decodeOn checks one rule against a field of a place.
func (a AuthoredScaling) decodeOn(part int, f Field, s Scalable) (Scaling, error) {
	if err := a.shape(f); err != nil {
		return Scaling{}, err
	}
	class, err := a.class()
	if err != nil {
		return Scaling{}, err
	}
	r, err := a.rule(f, s)
	if err != nil {
		return Scaling{}, err
	}
	return Scaling{Part: part, Field: a.Field, Axis: Axis(a.Axis), Class: class, Rule: r}, nil
}

// shape refuses an axis or a rule the rules do not know, or one the field cannot take.
func (a AuthoredScaling) shape(f Field) error {
	if err := oneOf("axis", a.Axis, Axes()); err != nil {
		return err
	}
	if err := oneOf("rule", a.Rule, []string{"linear", "table", "formula", "column"}); err != nil {
		return err
	}
	if f.Type != DiceField && f.Type != NumberField && a.Rule != "table" {
		return refuse("rule", "this field grows only by a table")
	}
	if (a.Axis == string(TableColumn)) != (a.Rule == "column") {
		return refuse("rule", "a class table's column scales by its column alone")
	}
	return nil
}

// class is the class a class level or a class table belongs to; the other axes have none.
func (a AuthoredScaling) class() (string, error) {
	if a.Axis != string(ClassLevel) && a.Axis != string(TableColumn) {
		return "", nil
	}
	if a.Class == "" || utf8.RuneCountInString(a.Class) > 80 {
		return "", refuse("class", "name the class in up to 80 letters")
	}
	return a.Class, nil
}

// rule makes the rule from its own values, each checked against the field.
func (a AuthoredScaling) rule(f Field, s Scalable) (Rule, error) {
	switch a.Rule {
	case "linear":
		return a.linear(f)
	case "formula":
		return a.formula(s)
	case "column":
		if a.Column == "" || utf8.RuneCountInString(a.Column) > 80 {
			return nil, refuse("column", "name the column in up to 80 letters")
		}
		return Column{Name: a.Column}, nil
	}
	return a.table(f)
}

func (a AuthoredScaling) linear(f Field) (Rule, error) {
	if a.Base < 0 || a.Base > 20 {
		return nil, refuse("base", "runs from 0 to 20")
	}
	per := Value{Text: "", Number: 0, Flag: false, Texts: nil}
	if a.Per != nil {
		per = *a.Per
	}
	if f.Type == DiceField {
		if !dicePattern.MatchString(per.Text) {
			return nil, refuse("per", "write the dice like 1d6")
		}
		return Linear{Base: a.Base, Per: textValue(per.Text)}, nil
	}
	if per.Number == 0 || per.Number < -1000 || per.Number > 1000 {
		return nil, refuse("per", "runs from -1000 to 1000, not 0")
	}
	return Linear{Base: a.Base, Per: numberValue(per.Number)}, nil
}

func (a AuthoredScaling) formula(s Scalable) (Rule, error) {
	if !s.Formula {
		return nil, refuse("rule", "this field takes no formula")
	}
	_, ok := Evaluate(a.Formula, 1)
	if !ok || len(a.Formula) > 60 || strings.Trim(a.Formula, "0123456789elv+-*() ") != "" {
		return nil, refuse("formula", "write a formula of level, like 11 + level")
	}
	return Formula{Expr: a.Formula}, nil
}

func (a AuthoredScaling) table(f Field) (Rule, error) {
	if len(a.Steps) == 0 || len(a.Steps) > 20 {
		return nil, refuse("steps", "give 1 to 20 steps, each at a higher level")
	}
	out := Table{Steps: make([]Step, 0, len(a.Steps))}
	last := 0
	for _, st := range a.Steps {
		if st.At <= last || st.At > 20 {
			return nil, refuse("steps", "give 1 to 20 steps, each at a higher level")
		}
		last = st.At
		v, err := stepValue(f, st.Value)
		if err != nil {
			return nil, err
		}
		out.Steps = append(out.Steps, Step{At: st.At, Value: v})
	}
	return out, nil
}

// stepValue is a step's value as its field holds it.
func stepValue(f Field, v Value) (Value, error) {
	switch f.Type {
	case DiceField:
		if !dicePattern.MatchString(v.Text) {
			return v, refuse("steps", "write the dice like 1d6")
		}
		return textValue(v.Text), nil
	case NumberField:
		if v.Number < f.Min || v.Number > f.Max {
			return v, refuse("steps", "runs from "+strconv.Itoa(f.Min)+" to "+strconv.Itoa(f.Max))
		}
		return numberValue(v.Number), nil
	case ChoiceField:
		return textValue(v.Text), oneOf("steps", v.Text, f.Options)
	case FlagField, TextField, ChoicesField:
	}
	return Value{Text: "", Number: 0, Flag: v.Flag, Texts: nil}, nil
}
