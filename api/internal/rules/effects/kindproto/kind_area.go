package kindproto

import (
	"errors"
	"strconv"
)

// Area is the template an Effect covers.
type Area struct {
	Shape   string
	SizeFt  int
	RangeFt int
}

type areaKind struct{}

func (areaKind) Describe() Description {
	return Description{
		Slug: "area", Label: "Area", Summary: "The template the parts after it cover.",
		Fields: []Field{
			{Name: "shape", Label: "Shape", Type: "enum", Options: []string{"sphere", "cone", "line", "cube", "cylinder", "emanation"}, Required: true},
			{Name: "size_ft", Label: "Size (ft)", Type: "int", Required: true, Min: 5, Max: 1000},
			{Name: "range_ft", Label: "Range (ft)", Type: "int", Min: 0, Max: 5000, Help: "0 starts at the caster."},
		},
		Table:    Table{Name: "effect_areas", Columns: []string{"shape text CHECK (shape IN (...))", "size_ft integer CHECK (size_ft BETWEEN 5 AND 1000)", "range_ft integer CHECK (range_ft BETWEEN 0 AND 5000)"}},
		Template: "A {size_ft}-foot {shape}{?range_ft: within {range_ft} feet}.",
		Files:    files("area"),
	}
}

func (areaKind) Decode(v Values) (Part, error) {
	a := Area{Shape: str(v, "shape"), SizeFt: number(v, "size_ft"), RangeFt: number(v, "range_ft")}
	if a.Shape == "" {
		return nil, errors.New("Choose a shape.")
	}
	if a.SizeFt < 5 {
		return nil, errors.New("Size is at least 5 feet.")
	}
	return a, nil
}

// Text is the rules sentence.
func (a Area) Text([]string) string {
	s := "A " + strconv.Itoa(a.SizeFt) + "-foot " + a.Shape
	if a.RangeFt > 0 {
		s += " within " + strconv.Itoa(a.RangeFt) + " feet"
	}
	return s + "."
}
