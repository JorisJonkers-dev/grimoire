package effects

// ResourceChange gives back (Delta above 0) or takes uses of a Resource when the Effect lands.
type ResourceChange struct {
	Resource string
	Delta    int
}

func (ResourceChange) isComponent() {}

// Kind is resource_change.
func (ResourceChange) Kind() string { return "resource_change" }

// Text is the rules sentence.
func (r ResourceChange) Text() string {
	if r.Delta > 0 {
		return "The target regains " + plural(r.Delta, "use") + " of " + title(r.Resource) + "."
	}
	return "The target expends " + plural(-r.Delta, "use") + " of " + title(r.Resource) + "."
}

// LandsOnBearer gives back or takes the uses.
func (r ResourceChange) LandsOnBearer() []Act {
	return []Act{ChangeUses(r)}
}

type resourceChangeKind struct{}

func (resourceChangeKind) Describe() Description {
	return Description{
		Slug: "resource_change", Label: "Resource uses", Summary: "Gives back or takes uses of one of the bearer's Resources as the Effect lands.",
		Fields: []Field{
			{Name: "resource", Label: "Resource", Type: TextField, Options: nil, Required: true, Min: 0, Max: 80},
			{Name: "delta", Label: "Uses given back, or taken when below 0", Type: NumberField, Options: nil, Required: true, Min: -20, Max: 20},
		},
		Groups: nil,
	}
}

func (k resourceChangeKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	resource, err := v.text(d.field("resource"))
	if err != nil {
		return nil, err
	}
	delta, err := v.number(d.field("delta"))
	if err != nil {
		return nil, err
	}
	if delta == 0 {
		return nil, refuse("delta", "give back or take at least one use")
	}
	return ResourceChange{Resource: resource, Delta: delta}, nil
}
