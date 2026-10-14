package effects

import "strconv"

// Teleport moves its caster up to RangeFt to an open hex (Misty Step).
type Teleport struct {
	RangeFt int
}

func (Teleport) isComponent() {}

// Kind is teleport.
func (Teleport) Kind() string { return "teleport" }

// Text is the rules sentence.
func (t Teleport) Text() string {
	return "The caster teleports up to " + strconv.Itoa(t.RangeFt) + " feet to an unoccupied space it can see."
}

// LandsOnBearer moves the caster, who is the Effect's bearer.
func (t Teleport) LandsOnBearer() []Act {
	return []Act{TeleportCaster(t)}
}

type teleportKind struct{}

func (teleportKind) Describe() Description {
	return Description{
		Slug: "teleport", Label: "Teleport", Summary: "Moves the caster to an unoccupied space it can see.",
		Fields: []Field{
			{Name: "range_ft", Label: "Range in feet", Type: NumberField, Options: nil, Required: true, Min: 5, Max: 5280},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k teleportKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("range_ft"))
	if err != nil {
		return nil, err
	}
	return Teleport{RangeFt: n}, nil
}
