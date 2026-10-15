package effects

import (
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// TargetKind is what an Activation picks.
type TargetKind string

// Target kinds; the zero kind states no Targeting, as an Effect only applied by hand.
const (
	TargetSelf      TargetKind = "self"
	TargetCreatures TargetKind = "creatures"
	TargetObject    TargetKind = "object"
	TargetPoint     TargetKind = "point"
	TargetArea      TargetKind = "area"
)

// Side is which side of the one activating a target must be on.
type Side string

// Sides.
const (
	AnySide Side = ""
	Allies  Side = "allies"
	Enemies Side = "enemies"
)

// Targeting is what an Activation may pick: its kind, how many, how far, from which side and of which
// creature types, whether each must be seen or willing, and how far each may be from the first. A
// creature on the same side counts as willing. A range of 0 is touch.
type Targeting struct {
	Kind     TargetKind
	Count    int
	RangeFt  int
	Side     Side
	Types    []string
	Seen     bool
	Willing  bool
	SpreadFt int
}

// Candidate is one pick as the Activation sees it: where it is, whether it is the one activating, on the
// same side or an object, its creature type, and whether the one activating sees it with its own Senses.
type Candidate struct {
	ID     string
	At     hex.Coord
	Self   bool
	Ally   bool
	Object bool
	Type   string
	Seen   bool
}

// Refusal is why a pick is not legal; Pick is its place among the picks, or -1 for the picks as a whole.
type Refusal struct {
	Pick   int
	Reason string
}

// TargetKinds lists every kind of Targeting.
func TargetKinds() []string {
	return []string{string(TargetSelf), string(TargetCreatures), string(TargetObject), string(TargetPoint), string(TargetArea)}
}

// Check refuses a Targeting the rules cannot hold: an unknown kind or side, or a count, distance or list
// of creature types out of range.
func (t Targeting) Check() error {
	switch {
	case !slices.Contains(TargetKinds(), string(t.Kind)):
		return refuse("kind", "choose one of the options")
	case t.Count < 0 || t.Count > 50:
		return refuse("count", "runs from 0 to 50")
	case t.RangeFt < 0 || t.RangeFt > 5280:
		return refuse("range_ft", "runs from 0 to 5280")
	case !slices.Contains([]Side{AnySide, Allies, Enemies}, t.Side):
		return refuse("side", "choose one of the options")
	case len(t.Types) > 14 || slices.Contains(t.Types, ""):
		return refuse("creature_types", "name up to 14 types")
	case t.SpreadFt < 0 || t.SpreadFt > 5280:
		return refuse("spread_ft", "runs from 0 to 5280")
	}
	return nil
}

// Refusals is every reason the picks are not legal for the Targeting, from where the one activating
// stands; none when they are.
func (t Targeting) Refusals(from hex.Coord, picks []Candidate) []Refusal {
	switch t.Kind {
	case TargetSelf:
		return t.self(picks)
	case TargetArea:
		if len(picks) > 0 {
			return []Refusal{{Pick: -1, Reason: "The area picks its own targets."}}
		}
		return nil
	case TargetCreatures, TargetObject, TargetPoint:
		return t.chosen(from, picks)
	}
	return nil
}

func (Targeting) self(picks []Candidate) []Refusal {
	var out []Refusal
	for i, c := range picks {
		if !c.Self {
			out = append(out, Refusal{Pick: i, Reason: "This affects only you."})
		}
	}
	return out
}

func (t Targeting) chosen(from hex.Coord, picks []Candidate) []Refusal {
	most := max(t.Count, 1)
	switch {
	case len(picks) == 0:
		return []Refusal{{Pick: -1, Reason: "Choose a target."}}
	case len(picks) > most:
		return []Refusal{{Pick: -1, Reason: "Choose at most " + strconv.Itoa(most) + " " + noun(most, "target") + "."}}
	}
	var out []Refusal
	for i, c := range picks {
		if reason := t.illegal(from, picks, i, c); reason != "" {
			out = append(out, Refusal{Pick: i, Reason: reason})
		}
	}
	return out
}

// illegal is why one pick is not legal, or nothing.
func (t Targeting) illegal(from hex.Coord, picks []Candidate, i int, c Candidate) string {
	switch {
	case slices.ContainsFunc(picks[:i], func(o Candidate) bool { return o.ID == c.ID }):
		return "Choose each target once."
	case hex.Distance(from, c.At)*hex.FeetPerHex > max(t.RangeFt, hex.FeetPerHex):
		return "Out of range: the range is " + t.reach() + "."
	case t.Kind == TargetPoint:
		return ""
	case t.Kind == TargetObject && !c.Object:
		return "Choose an object, not a creature."
	case t.Kind == TargetCreatures && c.Object:
		return "Choose a creature, not an object."
	}
	return t.creature(picks[0], c)
}

// creature is why a creature is not legal by side, type, sight, willingness or spread, or nothing.
func (t Targeting) creature(first, c Candidate) string {
	switch {
	case t.Side == Allies && !c.Ally:
		return "Choose an ally."
	case t.Side == Enemies && c.Ally:
		return "Choose an enemy."
	case len(t.Types) > 0 && !slices.Contains(t.Types, c.Type):
		return "Choose " + t.types() + "."
	case t.Seen && !c.Seen:
		return "Choose a creature you can see."
	case t.Willing && !c.Ally:
		return "Choose a willing creature."
	case t.SpreadFt > 0 && hex.Distance(first.At, c.At)*hex.FeetPerHex > t.SpreadFt:
		return "Choose a target within " + strconv.Itoa(t.SpreadFt) + " feet of the first."
	}
	return ""
}

// Text is the Targeting as a spell prints it: its range and its targets.
func (t Targeting) Text() string {
	switch t.Kind {
	case TargetSelf:
		return "Range: self."
	case TargetArea:
		return "Range: " + t.reach() + "."
	case TargetCreatures, TargetObject, TargetPoint:
		return "Range: " + t.reach() + ". Targets: " + t.targets() + "."
	}
	return ""
}

func (t Targeting) reach() string {
	if t.RangeFt == 0 {
		return "touch"
	}
	return strconv.Itoa(t.RangeFt) + " feet"
}

// targets names what may be picked, as many as may be.
func (t Targeting) targets() string {
	what := map[TargetKind]string{TargetObject: "object", TargetPoint: "point"}[t.Kind]
	if what == "" {
		what = map[Side]string{AnySide: "creature", Allies: "ally", Enemies: "enemy"}[t.Side]
	}
	if t.Willing {
		what = "willing " + what
	}
	out := article(what) + " " + what
	if t.Count > 1 {
		out = "up to " + strconv.Itoa(t.Count) + " " + noun(t.Count, what)
	}
	if t.Seen {
		out += " you can see"
	}
	if len(t.Types) > 0 {
		out += " that is " + t.types()
	}
	if t.SpreadFt > 0 {
		out += ", each within " + strconv.Itoa(t.SpreadFt) + " feet of the first"
	}
	return out
}

// types names the creature types allowed, each with its article.
func (t Targeting) types() string {
	named := make([]string, 0, len(t.Types))
	for _, ty := range t.Types {
		named = append(named, article(ty)+" "+ty)
	}
	return strings.Join(named, " or ")
}

// noun is a noun for n of it: enemy, enemies.
func noun(n int, noun string) string {
	switch {
	case n == 1:
		return noun
	case strings.HasSuffix(noun, "y"):
		return strings.TrimSuffix(noun, "y") + "ies"
	}
	return noun + "s"
}
