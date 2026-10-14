package effects

// SaveMode is what a SaveEdge does to a saving throw.
type SaveMode string

// Save modes.
const (
	SaveAdvantage    SaveMode = "advantage"
	SaveDisadvantage SaveMode = "disadvantage"
	SaveFails        SaveMode = "fail"
)

// SaveEdge changes the bearer's saving throws of one ability: advantage, disadvantage or a sure failure.
type SaveEdge struct {
	Ability string
	Mode    SaveMode
}

func (SaveEdge) isPart() {}

// Kind is save_edge.
func (SaveEdge) Kind() string { return "save_edge" }

// Text is the rules sentence.
func (s SaveEdge) Text() string {
	switch s.Mode {
	case SaveFails:
		return "The target automatically fails " + title(s.Ability) + " saving throws."
	case SaveAdvantage:
		return "The target has Advantage on " + title(s.Ability) + " saving throws."
	case SaveDisadvantage:
	}
	return "The target has Disadvantage on " + title(s.Ability) + " saving throws."
}

// FoldSave changes a saving throw of the edge's ability.
func (s SaveEdge) FoldSave(p *SaveProfile, h Held, ability string) {
	if s.Ability != ability {
		return
	}
	switch s.Mode {
	case SaveFails:
		p.Fails = true
	case SaveAdvantage:
		p.Advantages = append(p.Advantages, h.Name+": advantage")
	case SaveDisadvantage:
		p.Disadvantages = append(p.Disadvantages, h.Name+": disadvantage")
	}
}

func saveModes() []string {
	return []string{string(SaveAdvantage), string(SaveDisadvantage), string(SaveFails)}
}

type saveEdgeKind struct{}

func (saveEdgeKind) Describe() Description {
	return Description{
		Slug: "save_edge", Label: "Saving throw edge", Summary: "Gives the bearer's saving throws of one ability advantage, disadvantage or a sure failure.",
		Fields: []Field{
			{Name: "ability", Label: "Saving throw", Type: ChoiceField, Options: abilities(), Required: true, Min: 0, Max: 0},
			{Name: "mode", Label: "Edge", Type: ChoiceField, Options: saveModes(), Required: true, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k saveEdgeKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	ability, err := v.choice(d.field("ability"))
	if err != nil {
		return nil, err
	}
	mode, err := v.choice(d.field("mode"))
	if err != nil {
		return nil, err
	}
	return SaveEdge{Ability: ability, Mode: SaveMode(mode)}, nil
}
