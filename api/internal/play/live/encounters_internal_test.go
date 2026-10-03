package live

import (
	"slices"
	"testing"

	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

// fixed answers every IntN with the same value.
type fixed int

func (f fixed) IntN(int) int { return int(f) }

func TestResolveDrawsByModeAndFillsPoolsToTheBudget(t *testing.T) {
	t.Parallel()
	band := prep.Pool{ID: prep.PoolID{1}, Name: "Band", LevelMin: 3, LevelMax: 5, Difficulty: "moderate", Members: []prep.PoolMember{{Slug: "goblin", Weight: 1, Min: 0, Max: 20}}}
	p := prep.Prep{Pools: []prep.Pool{band}, XP: map[string]int{"goblin": 50}, Levels: []int{4, 4}}
	quiet := prep.Table{ChancePct: 100, Entries: []prep.Entry{{Weight: 3, Kind: prep.EntryNothing, Label: "Wind"}}}
	c := prep.Check{Mode: prep.ModeForce}
	resolveCheck(&c, p, quiet, nil, fixed(0), 0, -1)
	if c.Status != prep.CheckResolved || c.Outcome != prep.OutcomeNothing || c.EntryLabel != "" {
		t.Fatalf("forcing a table of Nothing = %+v", c)
	}
	pooled := prep.Table{ChancePct: 40, Entries: []prep.Entry{{Weight: 1, Kind: prep.EntryPool, PoolID: &band.ID}}}
	c = prep.Check{Mode: prep.ModeNormal, ChancePct: 40}
	resolveCheck(&c, p, pooled, nil, fixed(0), 40, -1)
	if c.ChanceRoll != 40 || c.Outcome != prep.OutcomeFight || c.EntryLabel != "Band" || !slices.Equal(c.Monsters, []prep.EntryMonster{{Slug: "goblin", Count: 15}}) {
		t.Fatalf("two level-4 characters face 750 XP of goblins = %+v", c)
	}
	c = prep.Check{Mode: prep.ModeNormal, ChancePct: 40}
	resolveCheck(&c, p, pooled, nil, fixed(0), 41, -1)
	if c.Outcome != prep.OutcomeNothing || c.EntryLabel != "" {
		t.Fatalf("a roll over the chance = %+v", c)
	}
	p.Levels = nil
	c = prep.Check{Mode: prep.ModePick}
	resolveCheck(&c, p, pooled, nil, fixed(0), 0, 0)
	if c.Outcome != prep.OutcomeNothing || c.EntryLabel != "Band (the party is outside its levels)" {
		t.Fatalf("a party with no characters counts as level 1 = %+v", c)
	}
}
