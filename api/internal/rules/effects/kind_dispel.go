package effects

// Dispel ends every spell on the bearer when the Effect lands (Dispel Magic).
type Dispel struct{}

func (Dispel) isComponent() {}

// Kind is dispel.
func (Dispel) Kind() string { return "dispel" }

// Text is the rules sentence.
func (Dispel) Text() string { return "Every spell on the target ends." }

// LandsOnBearer ends every spell on the bearer.
func (Dispel) LandsOnBearer() []Act {
	return []Act{EndSpells{}}
}

type dispelKind struct{}

func (dispelKind) Describe() Description {
	return Description{
		Slug: "dispel", Label: "Dispel", Summary: "Ends every spell on the bearer as the Effect lands.",
		Fields: nil, Groups: nil, OpenGroups: false,
	}
}

func (dispelKind) Decode(Values, Groups) (Part, error) { return Dispel{}, nil }
