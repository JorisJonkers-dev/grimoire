// Package dice parses dice notation and resolves rolls from faces. Randomness is injected.
package dice

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotation is returned for dice notation this package does not accept.
var ErrNotation = errors.New("dice: bad notation")

// ErrFace is returned for a face value a die cannot show.
var ErrFace = errors.New("dice: impossible face")

// Keep says which dice of a group count.
type Keep string

// Keep rules.
const (
	KeepAll     Keep = ""
	KeepHighest Keep = "kh"
	KeepLowest  Keep = "kl"
)

// MaxDice caps the dice in one group; MaxGroups caps the groups in one spec.
const (
	MaxDice   = 20
	MaxGroups = 6
)

// Group is a number of identical dice, optionally keeping only the highest or lowest few.
type Group struct {
	Count int
	Faces int
	// Sign is +1, or -1 for dice subtracted from the total (Bane).
	Sign  int
	Keep  Keep
	KeepN int
}

// Spec is what to roll.
type Spec struct {
	Groups []Group
}

// ValidFaces reports whether a die with this many faces exists.
func ValidFaces(faces int) bool {
	switch faces {
	case 4, 6, 8, 10, 12, 20, 100:
		return true
	default:
		return false
	}
}

// Parse reads notation such as "2d20kh1+1d4" or "1d8-1d4".
func Parse(notation string) (Spec, error) {
	s := strings.ReplaceAll(strings.ToLower(notation), " ", "")
	if s == "" {
		return Spec{}, ErrNotation
	}
	var spec Spec
	for s != "" {
		sign := 1
		switch s[0] {
		case '+':
			s = s[1:]
		case '-':
			sign, s = -1, s[1:]
		}
		end := strings.IndexAny(s, "+-")
		if end == -1 {
			end = len(s)
		}
		g, err := parseGroup(s[:end])
		if err != nil {
			return Spec{}, err
		}
		g.Sign = sign
		spec.Groups = append(spec.Groups, g)
		s = s[end:]
	}
	if len(spec.Groups) > MaxGroups {
		return Spec{}, fmt.Errorf("%w: at most %d groups", ErrNotation, MaxGroups)
	}
	return spec, nil
}

func parseGroup(term string) (Group, error) {
	countText, rest, ok := strings.Cut(term, "d")
	if !ok {
		return Group{}, fmt.Errorf("%w: %q is not dice", ErrNotation, term)
	}
	count := 1
	if countText != "" {
		n, err := strconv.Atoi(countText)
		if err != nil {
			return Group{}, fmt.Errorf("%w: %q", ErrNotation, term)
		}
		count = n
	}
	g := Group{Count: count, Faces: 0, Sign: 1, Keep: KeepAll, KeepN: 0}
	facesText := rest
	for _, k := range []Keep{KeepHighest, KeepLowest} {
		if before, after, found := strings.Cut(rest, string(k)); found {
			n, err := strconv.Atoi(after)
			if err != nil {
				return Group{}, fmt.Errorf("%w: %q", ErrNotation, term)
			}
			facesText, g.Keep, g.KeepN = before, k, n
		}
	}
	faces, err := strconv.Atoi(facesText)
	if err != nil {
		return Group{}, fmt.Errorf("%w: %q", ErrNotation, term)
	}
	g.Faces = faces
	return g, g.validate()
}

func (g Group) validate() error {
	switch {
	case g.Count < 1 || g.Count > MaxDice:
		return fmt.Errorf("%w: roll 1 to %d dice at a time", ErrNotation, MaxDice)
	case !ValidFaces(g.Faces):
		return fmt.Errorf("%w: there is no d%d", ErrNotation, g.Faces)
	case g.Keep != KeepAll && (g.KeepN < 1 || g.KeepN >= g.Count):
		return fmt.Errorf("%w: keep between 1 and %d dice", ErrNotation, g.Count-1)
	}
	return nil
}

// String writes the spec back in notation Parse accepts.
func (s Spec) String() string {
	var b strings.Builder
	for i, g := range s.Groups {
		switch {
		case g.Sign < 0:
			b.WriteString("-")
		case i > 0:
			b.WriteString("+")
		}
		fmt.Fprintf(&b, "%dd%d", g.Count, g.Faces)
		if g.Keep != KeepAll {
			fmt.Fprintf(&b, "%s%d", g.Keep, g.KeepN)
		}
	}
	return b.String()
}

// Dice is how many dice the spec rolls.
func (s Spec) Dice() int {
	n := 0
	for _, g := range s.Groups {
		n += g.Count
	}
	return n
}

// Kept marks which faces of a group count toward the total. Ties keep the earliest dice.
func (g Group) Kept(faces []int) []bool {
	kept := make([]bool, len(faces))
	n := g.KeepN
	if g.Keep == KeepAll {
		n = len(faces)
	}
	better := func(a, b int) bool {
		if g.Keep == KeepLowest {
			return a < b
		}
		return a > b
	}
	for range n {
		best := -1
		for i, f := range faces {
			if !kept[i] && (best == -1 || better(f, faces[best])) {
				best = i
			}
		}
		kept[best] = true
	}
	return kept
}

// GroupResult is one group's faces with the ones that count.
type GroupResult struct {
	Faces    []int
	Kept     []bool
	Subtotal int
}

// Result is a resolved roll.
type Result struct {
	Groups []GroupResult
	Total  int
}

// Resolve totals a spec from its faces plus a flat modifier; the faces may come from a player or a seeded roll.
func Resolve(s Spec, faces [][]int, modifier int) (Result, error) {
	if len(faces) != len(s.Groups) {
		return Result{}, fmt.Errorf("%w: expected %d groups", ErrFace, len(s.Groups))
	}
	out := Result{Groups: make([]GroupResult, 0, len(s.Groups)), Total: modifier}
	for i, g := range s.Groups {
		if len(faces[i]) != g.Count {
			return Result{}, fmt.Errorf("%w: group %d needs %d dice", ErrFace, i+1, g.Count)
		}
		for _, f := range faces[i] {
			if err := CheckFace(g.Faces, f); err != nil {
				return Result{}, err
			}
		}
		kept := g.Kept(faces[i])
		sub := 0
		for j, f := range faces[i] {
			if kept[j] {
				sub += f
			}
		}
		sub *= g.Sign
		out.Groups = append(out.Groups, GroupResult{Faces: faces[i], Kept: kept, Subtotal: sub})
		out.Total += sub
	}
	return out, nil
}

// CheckFace validates a face typed by a player.
func CheckFace(faces, value int) error {
	if value < 1 || value > faces {
		return fmt.Errorf("%w: a d%d shows 1 to %d, not %d", ErrFace, faces, faces, value)
	}
	return nil
}

// Source is the injected randomness: Intn returns a uniform integer in [0, n).
type Source interface {
	IntN(n int) int
}

// Face rolls one die.
func Face(src Source, faces int) int {
	return src.IntN(faces) + 1
}
