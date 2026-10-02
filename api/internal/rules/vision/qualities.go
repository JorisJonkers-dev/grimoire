package vision

import (
	"slices"
	"strconv"
	"strings"
)

// Quality keeps a creature or object from being seen for what it is.
type Quality string

// Visibility Qualities, as the rules reference names them.
const (
	Hidden    Quality = "hidden"
	Invisible Quality = "invisible"
	Disguised Quality = "disguised"
	Illusory  Quality = "illusory"
	Ethereal  Quality = "ethereal"
	Darkness  Quality = "darkness"
	Heavy     Quality = "heavy"
	Secret    Quality = "secret"
)

// Qualities lists every Quality in the reference's order.
func Qualities() []Quality {
	return []Quality{Hidden, Invisible, Disguised, Illusory, Ethereal, Darkness, Heavy, Secret}
}

// Sense is a way of perceiving.
type Sense string

// Senses.
const (
	Sight       Sense = "sight"
	Darkvision  Sense = "darkvision"
	Blindsight  Sense = "blindsight"
	Tremorsense Sense = "tremorsense"
	Truesight   Sense = "truesight"
)

// Senses lists every Sense in the reference's order.
func Senses() []Sense {
	return []Sense{Sight, Darkvision, Blindsight, Tremorsense, Truesight}
}

// Outcome is what one Sense does against one Quality.
type Outcome int

// Outcomes.
const (
	// No means the Sense never gets past the Quality.
	No Outcome = iota
	// Check means the Quality holds until a check beats it.
	Check
	// WithinRange means the Sense gets past the Quality within its range.
	WithinRange
	// OnGround is WithinRange, but only for a creature or object on the ground.
	OnGround
	// Sees means the Sense gets past the Quality wherever it reaches.
	Sees
)

// Against is the reference table: what a Sense does against a Quality.
func Against(q Quality, s Sense) Outcome {
	row := map[Quality][5]Outcome{
		Hidden:    {Check, Check, WithinRange, OnGround, Check},
		Invisible: {No, No, WithinRange, OnGround, Sees},
		Disguised: {Check, Check, No, No, Sees},
		Illusory:  {Check, Check, Sees, Sees, Sees},
		Ethereal:  {No, No, No, No, Sees},
		Darkness:  {No, Sees, WithinRange, OnGround, Sees},
		Heavy:     {No, No, WithinRange, OnGround, No},
		Secret:    {Check, Check, No, No, No},
	}[q]
	for i, sense := range Senses() {
		if sense == s {
			return row[i]
		}
	}
	return No
}

// Perceiver is one Sense with its range in feet; 0 reaches as far as the eye.
type Perceiver struct {
	Sense   Sense
	RangeFt int
}

// Target is what a perceiver looks at: its Qualities, the ones a check has beaten, how far away it is,
// and whether it stands on the ground.
type Target struct {
	Qualities  []Quality
	Beaten     []Quality
	DistanceFt int
	OnGround   bool
}

// Result is what the senses make of a target: whether they notice it, and whether they see its true
// form when it is Disguised.
type Result struct {
	Seen     bool
	TrueForm bool
}

// Perceive works out what a set of Senses makes of a target. The target is seen once every Quality
// but Disguised is overcome; a Disguised target shows its true form only once that is overcome too.
func Perceive(senses []Perceiver, t Target) Result {
	r := Result{Seen: true, TrueForm: true}
	for _, q := range t.Qualities {
		got := overcomes(senses, q, t)
		if q == Disguised {
			r.TrueForm = r.TrueForm && got
			continue
		}
		r.Seen = r.Seen && got
	}
	r.TrueForm = r.TrueForm && r.Seen
	return r
}

// overcomes reports whether any of the senses gets past one Quality of the target.
func overcomes(senses []Perceiver, q Quality, t Target) bool {
	for _, p := range senses {
		if gets(p, q, t) {
			return true
		}
	}
	return false
}

// gets reports whether one sense gets past one Quality of the target.
func gets(p Perceiver, q Quality, t Target) bool {
	inRange := p.RangeFt == 0 || t.DistanceFt <= p.RangeFt
	near := p.RangeFt > 0 && t.DistanceFt <= p.RangeFt
	switch Against(q, p.Sense) {
	case Sees:
		return inRange
	case WithinRange:
		return near
	case OnGround:
		return near && t.OnGround
	case Check:
		return inRange && slices.Contains(t.Beaten, q)
	case No:
	}
	return false
}

// Pierces describes what a Sense gets past, for a creature's entry: "Blindsight 30 ft: Invisible,
// Illusory" lists the Qualities it overcomes without a check.
func Pierces(s Sense, rangeFt int) string {
	var names []string
	for _, q := range Qualities() {
		if o := Against(q, s); o == Sees || o == WithinRange || o == OnGround {
			names = append(names, label(q))
		}
	}
	head := label(Quality(s)) + " " + strconv.Itoa(rangeFt) + " ft"
	if len(names) == 0 {
		return head + ": nothing without a check"
	}
	return head + ": " + strings.Join(names, ", ")
}

func label(q Quality) string {
	switch q {
	case Darkness:
		return "Darkness"
	case Heavy:
		return "Heavy obscurement"
	case Hidden, Invisible, Disguised, Illusory, Ethereal, Secret:
	}
	return strings.ToUpper(string(q[:1])) + string(q[1:])
}

// Describe writes a creature's Visibility for its entry: what its eyes need a check for and never see,
// then what each of its other Senses gets past, in the reference's order.
func Describe(ranges map[Sense]int) []string {
	var check, never []string
	for _, q := range Qualities() {
		switch Against(q, Sight) {
		case Check:
			check = append(check, label(q))
		case No:
			never = append(never, label(q))
		case WithinRange, OnGround, Sees:
		}
	}
	out := []string{"Sight: a check against " + strings.Join(check, ", ") + "; never " + strings.Join(never, ", ")}
	for _, s := range Senses()[1:] {
		if ft := ranges[s]; ft > 0 {
			out = append(out, Pierces(s, ft))
		}
	}
	return out
}
