package encounters_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/encounters"
)

// script answers IntN from a fixed list, checking every bound it was asked for.
type script struct {
	t      *testing.T
	values []int
	bounds []int
}

func (s *script) IntN(n int) int {
	s.bounds = append(s.bounds, n)
	if len(s.values) == 0 {
		s.t.Fatalf("no value left for IntN(%d)", n)
	}
	v := s.values[0]
	s.values = s.values[1:]
	return v
}

func TestDifficultiesHaveNames(t *testing.T) {
	for name, want := range map[string]encounters.Difficulty{"low": encounters.Low, "moderate": encounters.Moderate, "high": encounters.High} {
		if d, ok := encounters.ParseDifficulty(name); !ok || d != want || d.String() != name {
			t.Errorf("%s = %v %v", name, d, ok)
		}
	}
	if d, ok := encounters.ParseDifficulty("deadly"); ok || d != encounters.Moderate {
		t.Errorf("deadly = %v %v", d, ok)
	}
}

func TestBudgetAddsEachCharactersShare(t *testing.T) {
	for _, c := range []struct {
		levels []int
		d      encounters.Difficulty
		want   int
	}{
		{nil, encounters.Moderate, 0},
		{[]int{1}, encounters.Low, 50},
		{[]int{1, 1, 1, 1}, encounters.Moderate, 300},
		{[]int{3, 3}, encounters.High, 800},
		{[]int{5, 10, 20}, encounters.Moderate, 750 + 2300 + 13200},
		{[]int{0, 25}, encounters.High, 100 + 22000},
		{[]int{2, 19}, encounters.Low, 100 + 5500},
	} {
		if got := encounters.Budget(c.levels, c.d); got != c.want {
			t.Errorf("Budget(%v, %s) = %d, want %d", c.levels, c.d, got, c.want)
		}
	}
}

func TestTriggeredWhenTheRollIsWithinTheChance(t *testing.T) {
	if !encounters.Triggered(15, 15) || encounters.Triggered(16, 15) || !encounters.Triggered(1, 15) || encounters.Triggered(1, 0) {
		t.Error("triggered")
	}
}

func TestDrawFollowsTheWeights(t *testing.T) {
	weights := []int{0, 3, -2, 1}
	for n, want := range map[int]int{0: 1, 2: 1, 3: 3} {
		src := &script{t: t, values: []int{n}}
		if got := encounters.Draw(src, weights); got != want || !slices.Equal(src.bounds, []int{4}) {
			t.Errorf("Draw with %d = %d (bounds %v), want %d", n, got, src.bounds, want)
		}
	}
	if got := encounters.Draw(&script{t: t}, []int{0, -1}); got != -1 {
		t.Errorf("nothing to draw = %d", got)
	}
	if got := encounters.Draw(&script{t: t}, nil); got != -1 {
		t.Errorf("no weights = %d", got)
	}
}

func TestFillSpendsTheBudgetWithinEachMembersLimits(t *testing.T) {
	members := []encounters.Member{
		{Slug: "goblin", XP: 50, Weight: 3, Min: 1, Max: 4},
		{Slug: "hobgoblin", XP: 100, Weight: 1, Min: 0, Max: 1},
		{Slug: "ogre", XP: 450, Weight: 5, Min: 0, Max: 2},
	}
	src := &script{t: t, values: []int{3, 0, 0, 0}}
	got := encounters.Fill(src, 300, members)
	want := []encounters.Pick{{Slug: "goblin", Count: 4}, {Slug: "hobgoblin", Count: 1}}
	if !slices.Equal(got, want) || !slices.Equal(src.bounds, []int{4, 3, 3, 3}) {
		t.Fatalf("Fill = %v (bounds %v), want %v", got, src.bounds, want)
	}
	exact := encounters.Fill(&script{t: t, values: []int{0}}, 100, members[:1])
	if !slices.Equal(exact, []encounters.Pick{{Slug: "goblin", Count: 2}}) {
		t.Fatalf("a draw that lands exactly on the budget = %v", exact)
	}
	if got := encounters.Fill(&script{t: t}, 0, members[1:]); got != nil {
		t.Fatalf("nothing fits = %v", got)
	}
}
