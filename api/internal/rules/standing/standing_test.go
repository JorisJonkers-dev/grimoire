package standing_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
)

// Standing is kept as a score nobody at the table sees, and read as one of five tiers.
func TestAScoreReadsAsATier(t *testing.T) {
	t.Parallel()
	for score, want := range map[int]standing.Tier{
		-100: standing.Hostile, -60: standing.Hostile,
		-59: standing.Unfriendly, -20: standing.Unfriendly,
		-19: standing.Neutral, 0: standing.Neutral, 19: standing.Neutral,
		20: standing.Friendly, 59: standing.Friendly,
		60: standing.Allied, 100: standing.Allied,
	} {
		if got := standing.TierOf(score); got != want {
			t.Errorf("a score of %d is %s, want %s", score, got, want)
		}
	}
	if got := []standing.Tier{standing.Hostile, standing.Unfriendly, standing.Neutral, standing.Friendly, standing.Allied}; got[0] != "hostile" || got[1] != "unfriendly" || got[2] != "neutral" || got[3] != "friendly" || got[4] != "allied" {
		t.Errorf("tiers are named %v", got)
	}
}

// A Standing Change moves the score, never past either end.
func TestAChangeMovesTheScoreWithinItsBounds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ score, delta, want int }{
		{0, 25, 25}, {0, -25, -25}, {90, 25, 100}, {-90, -25, -100}, {100, 1, 100}, {-100, -1, -100}, {40, -80, -40}, {75, 25, 100}, {-75, -25, -100},
	} {
		if got := standing.Apply(c.score, c.delta); got != c.want {
			t.Errorf("%d moved by %d = %d, want %d", c.score, c.delta, got, c.want)
		}
	}
	for delta, want := range map[int]bool{1: true, -1: true, 100: true, -100: true, 0: false, 101: false, -101: false} {
		if got := standing.ValidDelta(delta); got != want {
			t.Errorf("a change of %d allowed = %v", delta, got)
		}
	}
	if standing.Min != -100 || standing.Max != 100 {
		t.Errorf("bounds %d %d", standing.Min, standing.Max)
	}
}

// The catalogue holds the generic archetypes a DM copies and names, each findable by its slug.
func TestTheArchetypeCatalogue(t *testing.T) {
	t.Parallel()
	all := standing.Archetypes()
	want := []string{"arcane-college", "city-watch", "cult", "druid-circle", "knightly-order", "mercenary-company", "merchant-league", "noble-house", "smuggling-ring", "temple", "thieves-guild"}
	if len(all) != len(want) {
		t.Fatalf("%d archetypes", len(all))
	}
	for i, a := range all {
		if a.Slug != want[i] || a.Name == "" || len(a.Description) < 40 || len(a.Goals) < 20 {
			t.Errorf("archetype %d = %+v", i, a)
		}
		got, ok := standing.Archetype(a.Slug)
		if !ok || got != a {
			t.Errorf("%s is not found by its slug", a.Slug)
		}
	}
	if _, ok := standing.Archetype("harpers"); ok {
		t.Error("a faction of a published setting is in the catalogue")
	}
	// The catalogue handed out is a copy: changing it changes nothing.
	all[0].Name = "Changed"
	if standing.Archetypes()[0].Name == "Changed" {
		t.Error("the catalogue can be changed from outside")
	}
}
