package effects

// Reacts lets the bearer take a reaction when Trigger happens (damaged: a creature it can see damages
// it); the DM resolves what it does from Instruction.
type Reacts struct {
	Trigger     string
	Instruction string
}

func (Reacts) isComponent() {}

// Kind is reacts.
func (Reacts) Kind() string { return "reacts" }

// Text is the rules sentence.
func (r Reacts) Text() string {
	return "When the target is " + r.Trigger + ", it can take a Reaction: " + r.Instruction
}

// FoldActions gives the bearer the reaction when its trigger happens.
func (r Reacts) FoldActions(p *ActionProfile, h Held) {
	if p.Reactions == nil {
		p.Reactions = map[string][]Reaction{}
	}
	p.Reactions[r.Trigger] = append(p.Reactions[r.Trigger], Reaction{Name: h.Name, Instruction: r.Instruction})
}

type reactsKind struct{}

func (reactsKind) Describe() Description {
	return Description{
		Slug: "reacts", Label: "Reaction", Summary: "Lets the bearer take a reaction, which the DM resolves, when something happens to it.",
		Fields: []Field{
			{Name: "trigger", Label: "When the bearer is", Type: ChoiceField, Options: []string{"damaged"}, Required: true, Min: 0, Max: 0},
			{Name: "instruction", Label: "What it does", Type: TextField, Options: nil, Required: true, Min: 0, Max: 500},
		},
		Groups: nil,
	}
}

func (k reactsKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	trigger, err := v.choice(d.field("trigger"))
	if err != nil {
		return nil, err
	}
	instruction, err := v.text(d.field("instruction"))
	if err != nil {
		return nil, err
	}
	return Reacts{Trigger: trigger, Instruction: instruction}, nil
}
