package effects

import "strconv"

// Form overlays the bearer's stat block with a creature's (Polymorph, Wild Shape). Monster empty lets
// whoever applies it choose the creature. The form's hit points are Temporary Hit Points: TempHP of
// them, or the creature's Hit Point maximum when TempHP is 0. The bearer reverts when they are gone.
type Form struct {
	Monster string
	TempHP  int
}

func (Form) isPart() {}

// Kind is form.
func (Form) Kind() string { return "form" }

// Text says what the target becomes and what carries its hit points.
func (f Form) Text() string {
	what := "a creature of the caster's choice"
	if f.Monster != "" {
		what = article(f.Monster) + " " + title(f.Monster)
	}
	pool := "Temporary Hit Points equal to that creature's Hit Point maximum"
	if f.TempHP > 0 {
		pool = strconv.Itoa(f.TempHP) + " Temporary Hit Points"
	}
	return "The target takes the form of " + what + ", using its statistics and gaining " + pool +
		"; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points."
}

// LandsOnBearer turns the bearer into the form.
func (f Form) LandsOnBearer() []Act {
	return []Act{TakeForm(f)}
}

type formKind struct{}

func (formKind) Describe() Description {
	return Description{
		Slug: "form", Label: "Take a form", Summary: "Overlays the bearer's statistics with a creature's until its hit points run out.",
		Fields: []Field{
			{Name: "monster", Label: "Creature, or empty for the caster's choice", Type: TextField, Options: nil, Required: false, Min: 0, Max: 80},
			{Name: "temp_hp", Label: "Temporary Hit Points, or 0 for the creature's maximum", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 999},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k formKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	monster, err := v.optionalText(d.field("monster"))
	if err != nil {
		return nil, err
	}
	hp, err := v.number(d.field("temp_hp"))
	if err != nil {
		return nil, err
	}
	return Form{Monster: monster, TempHP: hp}, nil
}
