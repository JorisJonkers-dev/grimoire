package effects

import (
	"slices"
	"strconv"
	"strings"
)

// Scaled is the field a Scaling grows, as its Part's kind or the Activation describes it; ok is false
// when neither describes it.
func (d Definition) Scaled(s Scaling) (Field, bool) {
	fields := ActivationFields()
	if s.Part != OnActivation {
		fields = nil
		at := 0
		walk(d.Parts, func(p Part) {
			if at == s.Part {
				k, _ := KindOf(p.Kind())
				fields = k.Describe().Fields
			}
			at++
		})
	}
	i := slices.IndexFunc(fields, func(f Field) bool { return f.Name == s.Field })
	if i >= 0 {
		return fields[i], true
	}
	return Field{Name: s.Field, Label: strings.ReplaceAll(s.Field, "_", " "), Type: "", Options: nil, Required: false, Min: 0, Max: 0}, false
}

// Text writes a Scaling of a field as a rules line.
func (s Scaling) Text(f Field) string {
	what := "the " + lower(f.Label)
	var line string
	switch r := s.Rule.(type) {
	case Linear:
		line = what + " increases by " + valueText(f, r.Per) + " for each " + s.unit() + " above " + strconv.Itoa(r.Base) + "."
	case Table:
		at := make([]string, 0, len(r.Steps))
		for _, st := range r.Steps {
			at = append(at, valueText(f, st.Value)+" "+s.at(st.At))
		}
		line = what + " becomes " + strings.Join(at, ", ") + "."
	case Formula:
		line = what + " is " + r.Expr + ", where level is the " + s.levelName() + "."
	case Column:
		line = what + " equals the " + r.Name + " column of the " + title(s.Class) + " table."
	}
	return s.lead(line)
}

// lead opens a line the way the SRD names the axis, or capitalises it.
func (s Scaling) lead(line string) string {
	switch s.Axis {
	case SlotLevel:
		return "Using a Higher-Level Spell Slot: " + line
	case CharacterLevel:
		return "Cantrip Upgrade: " + line
	case ClassLevel, TableColumn, Charges:
	}
	return strings.ToUpper(line[:1]) + line[1:]
}

// unit is what a linear rule counts levels in.
func (s Scaling) unit() string {
	switch s.Axis {
	case SlotLevel:
		return "slot level"
	case ClassLevel:
		return title(s.Class) + " level"
	case Charges:
		return "Charge spent"
	case CharacterLevel, TableColumn:
	}
	return "level"
}

// at is where a step of a table starts.
func (s Scaling) at(n int) string {
	if s.Axis == Charges {
		return "with " + strconv.Itoa(n) + " Charges"
	}
	return "at " + s.unit() + " " + strconv.Itoa(n)
}

// levelName is what level stands for in a formula.
func (s Scaling) levelName() string {
	switch s.Axis {
	case CharacterLevel:
		return "character level"
	case Charges:
		return "number of Charges spent"
	case SlotLevel, ClassLevel, TableColumn:
	}
	return s.unit()
}

// valueText writes a value as its field holds it.
func valueText(f Field, v Value) string {
	switch f.Type {
	case NumberField:
		return strconv.Itoa(v.Number)
	case FlagField:
		if v.Flag {
			return "on"
		}
		return "off"
	case DiceField, TextField, ChoiceField, ChoicesField:
	}
	return v.Text
}
