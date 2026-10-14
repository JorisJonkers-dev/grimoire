package effects

import "strconv"

// Counter lets the bearer counter, with its reaction, a spell cast within RangeFt (Counterspell).
type Counter struct {
	RangeFt int
}

func (Counter) isPart() {}

// Kind is counter.
func (Counter) Kind() string { return "counter" }

// Text is the rules sentence.
func (c Counter) Text() string {
	return "As a Reaction, the caster interrupts a creature it can see casting a spell within " + strconv.Itoa(c.RangeFt) + " feet; the spell fails."
}

// FoldActions gives the bearer the counter when it reaches further than any it has; the first of
// equal counters keeps its name.
func (c Counter) FoldActions(p *ActionProfile, h Held) {
	if c.RangeFt > p.CounterFt {
		p.CounterFt, p.CounterBy = c.RangeFt, h.Name
	}
}

type counterKind struct{}

func (counterKind) Describe() Description {
	return Description{
		Slug: "counter", Label: "Counter a spell", Summary: "Lets the bearer use its reaction to make a spell cast nearby fail.",
		Fields: []Field{
			{Name: "range_ft", Label: "Range in feet", Type: NumberField, Options: nil, Required: true, Min: 5, Max: 5280},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k counterKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("range_ft"))
	if err != nil {
		return nil, err
	}
	return Counter{RangeFt: n}, nil
}
