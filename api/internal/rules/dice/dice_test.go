package dice_test

import (
	"errors"
	"strings"
	"testing"
	"testing/quick"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func TestParseAndFormat(t *testing.T) {
	t.Parallel()
	cases := [][2]string{
		{"2d20kh1+1d4", "2d20kh1+1d4"},
		{"d20", "1d20"},
		{" 4D6 kh3 ", "4d6kh3"},
		{"2d20kl1-1d4", "2d20kl1-1d4"},
		{"+1d8", "1d8"},
		{"-1d4", "-1d4"},
		{"1d100", "1d100"},
	}
	for _, c := range cases {
		in, want := c[0], c[1]
		s, err := dice.Parse(in)
		if err != nil || s.String() != want {
			t.Errorf("Parse(%q) = %q %v, want %q", in, s.String(), err, want)
		}
	}
	for _, bad := range []string{"", "5", "1d7", "0d6", "21d6", "xd6", "1dx", "2d20kh", "2d20kh2", "2d20kl0", "1d20kh1", "1d4+1d4+1d4+1d4+1d4+1d4+1d4", "1d20+"} {
		if _, err := dice.Parse(bad); !errors.Is(err, dice.ErrNotation) {
			t.Errorf("Parse(%q) accepted: %v", bad, err)
		}
	}
	if _, err := dice.Parse("2d20kh2"); err == nil || !strings.Contains(err.Error(), "keep between 1 and 1 dice") {
		t.Errorf("keep message = %v", err)
	}
	if _, err := dice.Parse("21d6"); err == nil || !strings.Contains(err.Error(), "roll 1 to 20 dice") {
		t.Errorf("count message = %v", err)
	}
	s, _ := dice.Parse("2d20kh1+1d4")
	if s.Dice() != 3 {
		t.Fatalf("dice = %d", s.Dice())
	}
}

func TestResolveKeepsAndSubtracts(t *testing.T) {
	t.Parallel()
	s, _ := dice.Parse("2d20kh1+1d4-1d6")
	r, err := dice.Resolve(s, [][]int{{7, 15}, {3}, {2}}, 5)
	if err != nil || r.Total != 15+3-2+5 || r.Groups[0].Kept[0] || !r.Groups[0].Kept[1] || r.Groups[2].Subtotal != -2 {
		t.Fatalf("result = %+v %v", r, err)
	}
	low, _ := dice.Parse("2d20kl1")
	if r, _ := dice.Resolve(low, [][]int{{12, 12}}, 0); !r.Groups[0].Kept[0] || r.Groups[0].Kept[1] || r.Total != 12 {
		t.Fatalf("tie keeps the first = %+v", r)
	}
	if _, err := dice.Resolve(s, [][]int{{7, 15}, {3, 3}, {2}}, 0); err == nil || !strings.Contains(err.Error(), "group 2 needs 1 dice") {
		t.Errorf("group message = %v", err)
	}
	ties := []struct {
		notation string
		faces    []int
		kept     []bool
	}{
		{"2d20kh1", []int{12, 12}, []bool{true, false}},
		{"3d20kl2", []int{9, 9, 3}, []bool{true, false, true}},
		{"3d20kh2", []int{4, 9, 9}, []bool{false, true, true}},
		{"4d6kh3", []int{6, 1, 6, 6}, []bool{true, false, true, true}},
	}
	for _, c := range ties {
		spec, _ := dice.Parse(c.notation)
		got := spec.Groups[0].Kept(c.faces)
		for i := range got {
			if got[i] != c.kept[i] {
				t.Errorf("%s %v kept %v, want %v", c.notation, c.faces, got, c.kept)
				break
			}
		}
	}
	for _, faces := range [][][]int{{{7, 15}}, {{7}, {3}, {2}}, {{7, 21}, {3}, {2}}, {{7, 15}, {0}, {2}}} {
		if _, err := dice.Resolve(s, faces, 0); !errors.Is(err, dice.ErrFace) {
			t.Errorf("faces %v accepted: %v", faces, err)
		}
	}
}

func TestCheckFaceAndRoll(t *testing.T) {
	t.Parallel()
	if dice.CheckFace(6, 6) != nil || dice.CheckFace(6, 7) == nil || dice.CheckFace(6, 0) == nil {
		t.Fatal("check face")
	}
	src := newSource(1)
	seen := map[int]bool{}
	for range 400 {
		f := dice.Face(src, 6)
		if f < 1 || f > 6 {
			t.Fatalf("face %d", f)
		}
		seen[f] = true
	}
	if len(seen) != 6 {
		t.Fatalf("faces seen = %v", seen)
	}
	for _, f := range []int{4, 6, 8, 10, 12, 20, 100} {
		if !dice.ValidFaces(f) {
			t.Errorf("d%d", f)
		}
	}
	if dice.ValidFaces(3) {
		t.Error("d3")
	}
}

// source is a small deterministic generator for tests; the rules engine never picks its own randomness.
type source struct{ state uint64 }

func newSource(seed uint64) *source { return &source{state: seed*2862933555777941757 + 3037000493} }

func (s *source) IntN(n int) int {
	s.state ^= s.state << 13
	s.state ^= s.state >> 7
	s.state ^= s.state << 17
	return int(s.state % uint64(n)) //nolint:gosec // n is a small positive die size
}

func genSpec(r *source) dice.Spec {
	faces := []int{4, 6, 8, 10, 12, 20, 100}
	n := 1 + r.IntN(dice.MaxGroups)
	var s dice.Spec
	for i := range n {
		g := dice.Group{Count: 1 + r.IntN(dice.MaxDice), Faces: faces[r.IntN(len(faces))], Sign: 1}
		if i > 0 && r.IntN(4) == 0 {
			g.Sign = -1
		}
		if g.Count > 1 && r.IntN(2) == 0 {
			g.Keep = []dice.Keep{dice.KeepHighest, dice.KeepLowest}[r.IntN(2)]
			g.KeepN = 1 + r.IntN(g.Count-1)
		}
		s.Groups = append(s.Groups, g)
	}
	return s
}

func rollAll(s dice.Spec, r *source) [][]int {
	out := make([][]int, len(s.Groups))
	for i, g := range s.Groups {
		for range g.Count {
			out[i] = append(out[i], dice.Face(r, g.Faces))
		}
	}
	return out
}

func groupHolds(g dice.Group, faces []int, res dice.GroupResult) bool {
	kept, keptMin, droppedMax := 0, 1<<30, 0
	for j, k := range res.Kept {
		if k {
			kept++
			keptMin = min(keptMin, faces[j])
		} else {
			droppedMax = max(droppedMax, faces[j])
		}
	}
	n := g.Count
	if g.Keep != dice.KeepAll {
		n = g.KeepN
	}
	if kept != n || (g.Keep == dice.KeepHighest && droppedMax > keptMin) {
		return false
	}
	sub := abs(res.Subtotal)
	return sub >= n && sub <= n*g.Faces
}

func TestPropertiesOfNotationAndKeeping(t *testing.T) {
	t.Parallel()
	check := func(seed uint64) bool {
		r := newSource(seed)
		s := genSpec(r)
		back, err := dice.Parse(s.String())
		if err != nil || back.String() != s.String() {
			return false
		}
		faces := rollAll(s, r)
		res, err := dice.Resolve(s, faces, 0)
		if err != nil {
			return false
		}
		for i, g := range s.Groups {
			if !groupHolds(g, faces[i], res.Groups[i]) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(check, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatal(err)
	}
}

func TestKeepLowestNeverBeatsKeepHighest(t *testing.T) {
	t.Parallel()
	check := func(seed uint64, count uint8) bool {
		n := 2 + int(count)%(dice.MaxDice-1)
		r := newSource(seed)
		hi := dice.Group{Count: n, Faces: 20, Sign: 1, Keep: dice.KeepHighest, KeepN: 1}
		lo := hi
		lo.Keep = dice.KeepLowest
		faces := make([]int, n)
		for i := range faces {
			faces[i] = dice.Face(r, 20)
		}
		a, _ := dice.Resolve(dice.Spec{Groups: []dice.Group{hi}}, [][]int{faces}, 0)
		b, _ := dice.Resolve(dice.Spec{Groups: []dice.Group{lo}}, [][]int{faces}, 0)
		return b.Total <= a.Total
	}
	if err := quick.Check(check, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatal(err)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
