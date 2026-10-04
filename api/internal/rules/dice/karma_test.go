package dice_test

import (
	"testing"
	"testing/quick"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// script is a source that hands out the faces it was given, in order, and counts what was drawn.
type script struct {
	faces []int
	drawn int
}

func (s *script) IntN(int) int {
	s.drawn++
	return s.faces[s.drawn-1] - 1
}

func TestKarmaLeansAfterARunOfLowOrHighFaces(t *testing.T) {
	t.Parallel()
	cases := []struct {
		recent []int
		want   int
	}{
		{nil, 0},
		{[]int{3}, 0},
		{[]int{3, 5}, 1},
		{[]int{5, 5}, 1},
		{[]int{6, 5}, 0},
		{[]int{5, 6}, 0},
		{[]int{1, 1, 20}, 1},
		{[]int{20, 1, 1}, 0},
		{[]int{16, 20}, -1},
		{[]int{16, 16, 1}, -1},
		{[]int{15, 16}, 0},
		{[]int{16, 15}, 0},
		{[]int{5, 16}, 0},
		{[]int{20, 3}, 0},
		{[]int{10, 11}, 0},
	}
	for _, c := range cases {
		if got := dice.Karma(c.recent); got != c.want {
			t.Errorf("Karma(%v) = %d, want %d", c.recent, got, c.want)
		}
	}
}

func TestKarmicFaceRollsTwiceAndKeepsTheOneItLeansTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		faces         []int
		lean          int
		kept, dropped int
		drawn         int
	}{
		{[]int{7, 15}, 0, 7, 0, 1},
		{[]int{7, 15}, 1, 15, 7, 2},
		{[]int{15, 7}, 1, 15, 7, 2},
		{[]int{7, 15}, -1, 7, 15, 2},
		{[]int{15, 7}, -1, 7, 15, 2},
		{[]int{9, 9}, 1, 9, 9, 2},
		{[]int{9, 9}, -1, 9, 9, 2},
		{[]int{20, 1}, 5, 20, 1, 2},
		{[]int{20, 1}, -5, 1, 20, 2},
	}
	for _, c := range cases {
		src := &script{faces: c.faces, drawn: 0}
		kept, dropped := dice.KarmicFace(src, 20, c.lean)
		if kept != c.kept || dropped != c.dropped || src.drawn != c.drawn {
			t.Errorf("KarmicFace(%v, lean %d) = %d, %d after %d draws, want %d, %d after %d", c.faces, c.lean, kept, dropped, src.drawn, c.kept, c.dropped, c.drawn)
		}
	}
}

// The same seed rolls the same face with or without karmic dice until a run makes it lean, and a
// leaning roll is the same every time its seed is replayed.
func TestKarmicFaceKeepsSeededDeterminism(t *testing.T) {
	t.Parallel()
	property := func(seed uint64, lean int8) bool {
		plain := dice.Face(newSource(seed), 20)
		kept, dropped := dice.KarmicFace(newSource(seed), 20, 0)
		if kept != plain || dropped != 0 {
			return false
		}
		a, b := newSource(seed), newSource(seed)
		first, second := dice.Face(a, 20), dice.Face(a, 20)
		kept, dropped = dice.KarmicFace(b, 20, int(lean))
		again, againDropped := dice.KarmicFace(newSource(seed), 20, int(lean))
		if kept != again || dropped != againDropped {
			return false
		}
		switch {
		case lean > 0:
			return kept == max(first, second) && dropped == min(first, second)
		case lean < 0:
			return kept == min(first, second) && dropped == max(first, second)
		default:
			return kept == first && dropped == 0
		}
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}
