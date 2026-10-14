package effects

// GrantFeature gives the bearer a feature while the Effect lasts.
type GrantFeature struct {
	Name string
}

func (GrantFeature) isComponent() {}

// Kind is grant_feature.
func (GrantFeature) Kind() string { return "grant_feature" }

// Text is the rules sentence.
func (g GrantFeature) Text() string { return "The target gains " + g.Name + "." }

// LandsOnBearer gives the bearer the feature.
func (g GrantFeature) LandsOnBearer() []Act {
	return []Act{Grant{Feature: g.Name}}
}

type grantFeatureKind struct{}

func (grantFeatureKind) Describe() Description {
	return Description{
		Slug: "grant_feature", Label: "Grant a feature", Summary: "Gives the bearer a feature while the Effect lasts.",
		Fields: []Field{
			{Name: "name", Label: "Feature", Type: TextField, Options: nil, Required: true, Min: 0, Max: 120},
		},
		Groups: nil,
	}
}

func (k grantFeatureKind) Decode(v Values, _ Groups) (Part, error) {
	name, err := v.text(k.Describe().field("name"))
	if err != nil {
		return nil, err
	}
	return GrantFeature{Name: name}, nil
}
