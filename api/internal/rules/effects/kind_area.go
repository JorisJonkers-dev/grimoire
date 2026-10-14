package effects

import (
	"strconv"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// Area is an effect's template; RangeFt is how far away its point may be, 0 when it starts at the caster.
type Area struct {
	Shape   hex.Shape
	SizeFt  int
	RangeFt int
}

func (Area) isPart() {}

// Kind is area.
func (Area) Kind() string { return "area" }

// Text says where the template lies.
func (a Area) Text() string {
	size := strconv.Itoa(a.SizeFt) + "-foot " + title(string(a.Shape))
	switch {
	case a.Shape == hex.EmanationArea:
		return "A " + size + " surrounds the caster and moves with it."
	case a.RangeFt == 0:
		return "A " + size + " extends from the caster."
	}
	return "A " + size + " appears at a point within " + strconv.Itoa(a.RangeFt) + " feet."
}

// Asks for the template laid out on the board.
func (a Area) Asks() []Act {
	return []Act{Template{Area: a}}
}

func shapes() []string {
	return []string{
		string(hex.SphereArea), string(hex.CylinderArea), string(hex.EmanationArea), string(hex.RingArea),
		string(hex.ConeArea), string(hex.CubeArea), string(hex.LineArea), string(hex.WallArea),
	}
}

type areaKind struct{}

func (areaKind) Describe() Description {
	return Description{
		Slug: "area", Label: "Area", Summary: "The template an Effect fills: its shape, its size and how far away it may be placed.",
		Fields: []Field{
			{Name: "shape", Label: "Shape", Type: ChoiceField, Options: shapes(), Required: true, Min: 0, Max: 0},
			{Name: "size_ft", Label: "Size in feet", Type: NumberField, Options: nil, Required: true, Min: 5, Max: 500},
			{Name: "range_ft", Label: "Range in feet", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 5280},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k areaKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	shape, err := v.choice(d.field("shape"))
	if err != nil {
		return nil, err
	}
	size, err := v.number(d.field("size_ft"))
	if err != nil {
		return nil, err
	}
	reach, err := v.number(d.field("range_ft"))
	if err != nil {
		return nil, err
	}
	return Area{Shape: hex.Shape(shape), SizeFt: size, RangeFt: reach}, nil
}
