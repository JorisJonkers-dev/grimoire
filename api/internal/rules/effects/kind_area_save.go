package effects

// AreaSave is the saving throw every creature in an area makes, when what a failure brings is in
// Branches rather than in a SaveDamage or SaveCondition of its own.
type AreaSave struct {
	Ability string
}

func (AreaSave) isComponent() {}

// Kind is area_save.
func (AreaSave) Kind() string { return "area_save" }

// Text is the rules sentence.
func (s AreaSave) Text() string {
	return "Each creature in the area makes a " + title(s.Ability) + " saving throw."
}

// Asks for every creature's save.
func (s AreaSave) Asks() []Act {
	return []Act{RollSave(s)}
}

type areaSaveKind struct{}

func (areaSaveKind) Describe() Description {
	return Description{
		Slug: "area_save", Label: "Save in the area", Summary: "Each creature in the area makes a saving throw; what a failure brings sits in a branch.",
		Fields: []Field{
			{Name: "ability", Label: "Saving throw", Type: ChoiceField, Options: abilities(), Required: true, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k areaSaveKind) Decode(v Values, _ Groups) (Part, error) {
	ability, err := v.choice(k.Describe().field("ability"))
	if err != nil {
		return nil, err
	}
	return AreaSave{Ability: ability}, nil
}
