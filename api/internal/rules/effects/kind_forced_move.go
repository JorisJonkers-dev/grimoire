package effects

import "strconv"

// ForcedMove pushes creatures that fail the save Ft feet straight away from the origin, or pulls them
// toward it (Thunderwave).
type ForcedMove struct {
	Ft     int
	Toward bool
}

func (ForcedMove) isPart() {}

// Kind is forced_move.
func (ForcedMove) Kind() string { return "forced_move" }

// Text is the rules sentence.
func (f ForcedMove) Text() string {
	if f.Toward {
		return "A creature that fails is pulled up to " + strconv.Itoa(f.Ft) + " feet straight toward the caster."
	}
	return "A creature that fails is pushed up to " + strconv.Itoa(f.Ft) + " feet straight away from the caster."
}

// Asks for the push those who fail take.
func (f ForcedMove) Asks() []Act {
	return []Act{Push(f)}
}

type forcedMoveKind struct{}

func (forcedMoveKind) Describe() Description {
	return Description{
		Slug: "forced_move", Label: "Push or pull", Summary: "Moves each creature that fails straight away from the caster, or toward it.",
		Fields: []Field{
			{Name: "ft", Label: "Feet", Type: NumberField, Options: nil, Required: true, Min: 5, Max: 120},
			{Name: "toward", Label: "Pull toward the caster", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k forcedMoveKind) Decode(v Values, _ Groups) (Part, error) {
	ft, err := v.number(k.Describe().field("ft"))
	if err != nil {
		return nil, err
	}
	return ForcedMove{Ft: ft, Toward: v["toward"].Flag}, nil
}
