package effects

// Immobile holds the bearer's speed at zero.
type Immobile struct{}

func (Immobile) isComponent() {}

// Kind is immobile.
func (Immobile) Kind() string { return "immobile" }

// Text is the rules sentence.
func (Immobile) Text() string { return "The target's Speed is 0." }

// FoldMove holds the bearer's speed at zero.
func (Immobile) FoldMove(p *MoveProfile, _ Held) {
	p.Immobile = true
}

type immobileKind struct{}

func (immobileKind) Describe() Description {
	return Description{
		Slug: "immobile", Label: "Speed 0", Summary: "Holds the bearer's speed at zero.",
		Fields: nil, Groups: nil,
	}
}

func (immobileKind) Decode(Values, Groups) (Part, error) { return Immobile{}, nil }
