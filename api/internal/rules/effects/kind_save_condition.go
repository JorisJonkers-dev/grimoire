package effects

// SaveCondition puts a condition on every creature in the area that fails the save.
type SaveCondition struct {
	Ability string
	Slug    string
}

func (SaveCondition) isComponent() {}

// Kind is save_condition.
func (SaveCondition) Kind() string { return "save_condition" }

// Text is the rules sentence.
func (s SaveCondition) Text() string {
	return "On a failed " + title(s.Ability) + " saving throw, a creature has the " + title(s.Slug) + " condition."
}

// Asks for every creature's save, and the condition those who fail it take.
func (s SaveCondition) Asks() []Act {
	return []Act{RollSave{Ability: s.Ability}, Afflict{Condition: s.Slug}}
}

// LandsOnBearer puts the condition on, where a branch has already settled that the save failed.
func (s SaveCondition) LandsOnBearer() []Act {
	return []Act{Afflict{Condition: s.Slug}}
}

type saveConditionKind struct{}

func (saveConditionKind) Describe() Description {
	return Description{
		Slug: "save_condition", Label: "Save or take a condition", Summary: "Each creature in the area makes a saving throw or takes the condition.",
		Fields: []Field{
			{Name: "ability", Label: "Saving throw", Type: ChoiceField, Options: abilities(), Required: true, Min: 0, Max: 0},
			{Name: "condition", Label: "Condition", Type: TextField, Options: nil, Required: true, Min: 0, Max: 80},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k saveConditionKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	ability, err := v.choice(d.field("ability"))
	if err != nil {
		return nil, err
	}
	slug, err := v.text(d.field("condition"))
	if err != nil {
		return nil, err
	}
	return SaveCondition{Ability: ability, Slug: slug}, nil
}
