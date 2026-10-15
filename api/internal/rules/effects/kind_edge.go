package effects

import "slices"

// Range limits an advantage source by how far the attacker is from the bearer.
type Range int

// Ranges.
const (
	AnyRange Range = iota
	WithinFive
	BeyondFive
)

// Edge gives advantage or disadvantage: on the bearer's own attacks, or on attacks against the bearer.
// A SourceOnly edge against the bearer counts only for the Effect's source (Vex).
type Edge struct {
	Against    bool
	Advantage  bool
	Range      Range
	SourceOnly bool
}

func (Edge) isPart() {}

// Kind is edge.
func (Edge) Kind() string { return "edge" }

// Text is the rules sentence.
func (e Edge) Text() string {
	kind := map[bool]string{true: "Advantage", false: "Disadvantage"}[e.Advantage]
	text := "The target has " + kind + " on attack rolls"
	if e.Against {
		text = "Attack rolls against the target have " + kind
		if e.SourceOnly {
			text = "The source's attack rolls against the target have " + kind
		}
	}
	switch e.Range {
	case WithinFive:
		text += " from within 5 feet"
	case BeyondFive:
		text += " from more than 5 feet away"
	case AnyRange:
	}
	return text + "."
}

// FoldAttacking gives the bearer's own attacks the edge, whatever the range.
func (e Edge) FoldAttacking(p *AttackProfile, h Held) {
	if !e.Against {
		p.add(h.Name, e)
	}
}

// FoldAttacked gives an attack against the bearer the edge from the edge's range, and only for the
// Effect's source when it counts for the source alone.
func (e Edge) FoldAttacked(p *AttackProfile, h Held, by Attacker) {
	if e.Against && e.Range.covers(by.WithinFive) && (!e.SourceOnly || by.Source) {
		p.add(h.Name, e)
	}
}

func (r Range) covers(withinFive bool) bool {
	switch r {
	case WithinFive:
		return withinFive
	case BeyondFive:
		return !withinFive
	case AnyRange:
	}
	return true
}

func (p *AttackProfile) add(name string, e Edge) {
	if e.Advantage {
		p.Advantages = append(p.Advantages, name+": advantage")
		return
	}
	p.Disadvantages = append(p.Disadvantages, name+": disadvantage")
}

func rangeNames() []string {
	return []string{"any", "within_five", "beyond_five"}
}

type edgeKind struct{}

func (edgeKind) Describe() Description {
	return Description{
		Slug: "edge", Label: "Advantage or disadvantage", Summary: "Gives the bearer's attacks, or attacks against the bearer, advantage or disadvantage.",
		Fields: []Field{
			{Name: "against", Label: "On attacks against the bearer", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
			{Name: "advantage", Label: "Advantage", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
			{Name: "range", Label: "From", Type: ChoiceField, Options: rangeNames(), Required: true, Min: 0, Max: 0},
			{Name: "source_only", Label: "Only for the Effect's source", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (edgeKind) Hints() Hints {
	return Hints{
		Order:    []string{"advantage", "against", "range", "source_only"},
		Sections: []Section{{Label: "Which attacks", Fields: []string{"against", "range", "source_only"}}},
		Help: []Help{
			{Field: "advantage", Text: "Off: Disadvantage."},
			{Field: "against", Text: "Off: the bearer's own attacks."},
			{Field: "source_only", Text: "Only whoever applied the Effect, as with Vex."},
		},
	}
}

func (k edgeKind) Decode(v Values, _ Groups) (Part, error) {
	name, err := v.choice(k.Describe().field("range"))
	if err != nil {
		return nil, err
	}
	e := Edge{Against: v["against"].Flag, Advantage: v["advantage"].Flag, Range: Range(slices.Index(rangeNames(), name)), SourceOnly: v["source_only"].Flag}
	if e.SourceOnly && !e.Against {
		return nil, refuse("source_only", "only an edge on attacks against the bearer counts for its source alone")
	}
	return e, nil
}
