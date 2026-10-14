package effects

// Incapacitated takes away the bearer's actions, bonus actions and reactions.
type Incapacitated struct{}

func (Incapacitated) isComponent() {}

// Kind is incapacitated.
func (Incapacitated) Kind() string { return "incapacitated" }

// Text is the rules sentence.
func (Incapacitated) Text() string { return "The target has the Incapacitated condition." }

// FoldActions leaves the bearer no action, bonus action or reaction.
func (Incapacitated) FoldActions(p *ActionProfile, _ Held) {
	p.Incapacitated = true
}

type incapacitatedKind struct{}

func (incapacitatedKind) Describe() Description {
	return Description{
		Slug: "incapacitated", Label: "Incapacitated", Summary: "Takes away the bearer's actions, bonus actions and reactions.",
		Fields: nil, Groups: nil,
	}
}

func (incapacitatedKind) Decode(Values, Groups) (Part, error) { return Incapacitated{}, nil }
