package effects

import "strconv"

// MoveCost multiplies what each foot of movement costs the bearer (crawling while prone).
type MoveCost struct {
	Multiplier int
}

func (MoveCost) isComponent() {}

// Kind is move_cost.
func (MoveCost) Kind() string { return "move_cost" }

// Text is the rules sentence.
func (m MoveCost) Text() string {
	return "Every foot of movement costs " + strconv.Itoa(m.Multiplier) + " feet."
}

// FoldMove makes each foot cost the multiplier, when no other effect makes it cost more.
func (m MoveCost) FoldMove(p *MoveProfile, _ Held) {
	p.Multiplier = max(p.Multiplier, m.Multiplier)
}

type moveCostKind struct{}

func (moveCostKind) Describe() Description {
	return Description{
		Slug: "move_cost", Label: "Movement cost", Summary: "Makes each foot of the bearer's movement cost more.",
		Fields: []Field{
			{Name: "multiplier", Label: "Feet per foot", Type: NumberField, Options: nil, Required: true, Min: 1, Max: 4},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k moveCostKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("multiplier"))
	if err != nil {
		return nil, err
	}
	return MoveCost{Multiplier: n}, nil
}
