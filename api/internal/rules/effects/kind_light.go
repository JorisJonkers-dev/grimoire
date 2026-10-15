package effects

import "strconv"

// Light sheds bright light out to BrightFt and dim light DimFt beyond it from where the Effect lands.
type Light struct {
	BrightFt int
	DimFt    int
}

func (Light) isPart() {}

// Kind is light.
func (Light) Kind() string { return "light" }

// Text is the rules sentence.
func (l Light) Text() string {
	return "Bright light fills " + strconv.Itoa(l.BrightFt) + " feet around where it lands, and dim light another " + strconv.Itoa(l.DimFt) + " feet."
}

// Asks for the light shed where the area lands.
func (l Light) Asks() []Act {
	return []Act{ShedLight(l)}
}

type lightKind struct{}

func (lightKind) Describe() Description {
	return Description{
		Slug: "light", Label: "Light", Summary: "Sheds bright and dim light from where the Effect lands.",
		Fields: []Field{
			{Name: "bright_ft", Label: "Bright light in feet", Type: NumberField, Options: nil, Required: true, Min: 5, Max: 120},
			{Name: "dim_ft", Label: "Dim light beyond, in feet", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 120},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (lightKind) Hints() Hints {
	return Hints{Order: nil, Sections: nil, Help: []Help{{Field: "dim_ft", Text: "Beyond the bright light; 0 for none."}}}
}

func (k lightKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	bright, err := v.number(d.field("bright_ft"))
	if err != nil {
		return nil, err
	}
	dim, err := v.number(d.field("dim_ft"))
	if err != nil {
		return nil, err
	}
	return Light{BrightFt: bright, DimFt: dim}, nil
}
