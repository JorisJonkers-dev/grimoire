package effects

import "strconv"

// CritWithin makes every hit on the bearer from within Feet a Critical Hit.
type CritWithin struct {
	Feet int
}

func (CritWithin) isComponent() {}

// Kind is crit_within.
func (CritWithin) Kind() string { return "crit_within" }

// Text is the rules sentence.
func (c CritWithin) Text() string {
	return "Any hit against the target from within " + strconv.Itoa(c.Feet) + " feet is a Critical Hit."
}

// FoldAttacked turns a hit from within 5 feet into a Critical Hit, with one note however many effects do.
func (c CritWithin) FoldAttacked(p *AttackProfile, h Held, by Attacker) {
	if by.WithinFive && c.Feet >= 5 && !p.Crit {
		p.Crit = true
		p.Notes = append(p.Notes, h.Name+": a hit from this close is a Critical Hit")
	}
}

type critWithinKind struct{}

func (critWithinKind) Describe() Description {
	return Description{
		Slug: "crit_within", Label: "Critical Hits up close", Summary: "Makes every hit on the bearer from close by a Critical Hit.",
		Fields: []Field{
			{Name: "feet", Label: "Within feet", Type: NumberField, Options: nil, Required: true, Min: 0, Max: 120},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k critWithinKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("feet"))
	if err != nil {
		return nil, err
	}
	return CritWithin{Feet: n}, nil
}
