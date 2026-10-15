package designs

import (
	"errors"
	"slices"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// partOf reads one Part: kind, then each key as a field of that kind, its Scaling, or a group of Parts.
func partOf(n *yaml.Node) (effects.Authored, error) {
	a := effects.Authored{Kind: "", Values: effects.Values{}, Groups: nil, Scaling: nil}
	if n.Kind != yaml.MappingNode {
		return a, errors.New("write the Part as its fields")
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == "kind" {
			a.Kind = n.Content[i+1].Value
		}
	}
	if a.Kind == "" {
		return a, errors.New("kind: name the Part's kind")
	}
	var fields []effects.Field
	if k, ok := effects.KindOf(a.Kind); ok {
		fields = k.Describe().Fields
	}
	for i := 0; i < len(n.Content); i += 2 {
		if err := put(&a, n.Content[i].Value, n.Content[i+1], fields); err != nil {
			return a, err
		}
	}
	return a, nil
}

// put sets one key of a Part.
func put(a *effects.Authored, key string, v *yaml.Node, fields []effects.Field) error {
	f := slices.IndexFunc(fields, func(f effects.Field) bool { return f.Name == key })
	switch {
	case key == "kind":
		return nil
	case key == "scaling":
		var err error
		a.Scaling, err = scalingOf(v.Content, fields)
		return err
	case f >= 0:
		value, err := valueOf(v, fields[f])
		if err != nil {
			return effects.PartError{At: key, Err: err}
		}
		a.Values[key] = value
		return nil
	case v.Kind == yaml.SequenceNode:
		beneath, err := partsOf(v.Content)
		a.Groups = append(a.Groups, effects.AuthoredGroup{Name: key, Parts: beneath})
		if err != nil {
			return effects.PartError{At: key, Err: err}
		}
		return nil
	}
	a.Values[key] = effects.Value{Text: v.Value, Number: 0, Flag: false, Texts: nil}
	return nil
}

// valueOf reads a value as its field holds it.
func valueOf(n *yaml.Node, f effects.Field) (effects.Value, error) {
	out := effects.Value{Text: "", Number: 0, Flag: false, Texts: nil}
	switch f.Type {
	case effects.NumberField:
		if n.Kind != yaml.ScalarNode || n.Decode(&out.Number) != nil {
			return out, errors.New("write a whole number")
		}
	case effects.FlagField:
		if n.Kind != yaml.ScalarNode || n.Decode(&out.Flag) != nil {
			return out, errors.New("write true or false")
		}
	case effects.ChoicesField:
		if n.Kind != yaml.SequenceNode || n.Decode(&out.Texts) != nil {
			return out, errors.New("write a list of options")
		}
	case effects.DiceField, effects.TextField, effects.ChoiceField:
		if n.Kind != yaml.ScalarNode {
			return out, errors.New("write one value")
		}
		out.Text = n.Value
	}
	return out, nil
}

// scaling is a Scaling rule as a Design file writes it; per and each step's value are typed as the field
// the rule grows.
type scaling struct {
	Field   string    `yaml:"field"`
	Axis    string    `yaml:"axis"`
	Class   string    `yaml:"class"`
	Rule    string    `yaml:"rule"`
	Base    int       `yaml:"base"`
	Per     yaml.Node `yaml:"per"`
	Steps   []step    `yaml:"steps"`
	Formula string    `yaml:"formula"`
	Column  string    `yaml:"column"`
}

type step struct {
	At    int       `yaml:"at"`
	Value yaml.Node `yaml:"value"`
}

// scalingOf reads Scaling rules over fields; a value of a field that is not there is left for the kinds
// to refuse.
func scalingOf(nodes []*yaml.Node, fields []effects.Field) ([]effects.AuthoredScaling, error) {
	out := make([]effects.AuthoredScaling, 0, len(nodes))
	for i, n := range nodes {
		s, err := ruleOf(n, fields)
		if err != nil {
			return nil, effects.PartError{At: "scaling " + strconv.Itoa(i+1), Err: err}
		}
		out = append(out, s)
	}
	return out, nil
}

func ruleOf(n *yaml.Node, fields []effects.Field) (effects.AuthoredScaling, error) {
	var s scaling
	body, err := yaml.Marshal(n)
	if err == nil {
		err = strict(body, &s)
	}
	out := effects.AuthoredScaling{
		Field: s.Field, Axis: s.Axis, Class: s.Class, Rule: s.Rule, Base: s.Base, Per: nil, Steps: nil, Formula: s.Formula, Column: s.Column,
	}
	if err != nil {
		return out, err
	}
	f := effects.Field{Name: s.Field, Label: "", Type: effects.TextField, Options: nil, Required: false, Min: 0, Max: 0}
	if i := slices.IndexFunc(fields, func(f effects.Field) bool { return f.Name == s.Field }); i >= 0 {
		f = fields[i]
	}
	if !s.Per.IsZero() {
		per, err := valueOf(&s.Per, f)
		if err != nil {
			return out, effects.PartError{At: "per", Err: err}
		}
		out.Per = &per
	}
	for _, st := range s.Steps {
		v, err := valueOf(&st.Value, f)
		if err != nil {
			return out, effects.PartError{At: "steps", Err: err}
		}
		out.Steps = append(out.Steps, effects.AuthoredStep{At: st.At, Value: v})
	}
	return out, nil
}
