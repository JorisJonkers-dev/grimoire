package effects

import "strconv"

// SpeedPenalty takes feet off the bearer's speed (Slow); several do not add up.
type SpeedPenalty struct {
	Ft int
}

func (SpeedPenalty) isPart() {}

// Kind is speed_penalty.
func (SpeedPenalty) Kind() string { return "speed_penalty" }

// Text is the rules sentence.
func (s SpeedPenalty) Text() string {
	return "The target's Speed is reduced by " + strconv.Itoa(s.Ft) + " feet."
}

// FoldMove takes the feet off the bearer's speed, unless another penalty already takes more.
func (s SpeedPenalty) FoldMove(p *MoveProfile, _ Held) {
	p.LargestFt = max(p.LargestFt, s.Ft)
}

type speedPenaltyKind struct{}

func (speedPenaltyKind) Describe() Description {
	return Description{
		Slug: "speed_penalty", Label: "Speed penalty", Summary: "Takes feet off the bearer's speed; several do not add up.",
		Fields: []Field{
			{Name: "ft", Label: "Feet", Type: NumberField, Options: nil, Required: true, Min: 0, Max: 120},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (speedPenaltyKind) Hints() Hints {
	return Hints{Order: nil, Sections: nil, Help: []Help{{Field: "ft", Text: "Several penalties do not add up; the largest counts."}}}
}

func (k speedPenaltyKind) Decode(v Values, _ Groups) (Part, error) {
	n, err := v.number(k.Describe().field("ft"))
	if err != nil {
		return nil, err
	}
	return SpeedPenalty{Ft: n}, nil
}
