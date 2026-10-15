package effects

// Manual is a Manual Effect: a Part where someone must decide an outcome or change tracked state by
// hand, which the DM resolves from a prompt.
type Manual struct {
	Instruction string
}

func (Manual) isPart() {}

// Kind is manual.
func (Manual) Kind() string { return "manual" }

// Text is the instruction.
func (m Manual) Text() string { return m.Instruction }

// Asks the DM, once for the area.
func (m Manual) Asks() []Act {
	return []Act{AskDM(m)}
}

// LandsOnBearer asks the DM as the Effect lands.
func (m Manual) LandsOnBearer() []Act {
	return []Act{AskDM(m)}
}

type manualKind struct{}

func (manualKind) Describe() Description {
	return Description{
		Slug: "manual", Label: "Manual Effect", Summary: "An outcome the DM decides by hand from a prompt; play waits on it.",
		Fields: []Field{
			{Name: "instruction", Label: "Instruction", Type: TextField, Options: nil, Required: true, Min: 0, Max: 500},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (manualKind) Hints() Hints {
	return Hints{Order: nil, Sections: nil, Help: []Help{{Field: "instruction", Text: "What the DM decides when the Effect lands; play waits for it."}}}
}

func (k manualKind) Decode(v Values, _ Groups) (Part, error) {
	text, err := v.text(k.Describe().field("instruction"))
	if err != nil {
		return nil, err
	}
	return Manual{Instruction: text}, nil
}
