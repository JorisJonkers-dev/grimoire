package standing_test

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
)

var tiers = []standing.Tier{standing.Hostile, standing.Unfriendly, standing.Neutral, standing.Friendly, standing.Allied}

// A social check with a member of a Faction: Advantage when Allied, Disadvantage when Hostile, and a
// small bonus or penalty in between.
func TestStandingShapesSocialChecks(t *testing.T) {
	t.Parallel()
	want := map[standing.Tier]standing.Social{
		standing.Hostile:    {Mode: standing.Disadvantage, Bonus: 0},
		standing.Unfriendly: {Mode: standing.Straight, Bonus: -2},
		standing.Neutral:    {Mode: standing.Straight, Bonus: 0},
		standing.Friendly:   {Mode: standing.Straight, Bonus: 2},
		standing.Allied:     {Mode: standing.Advantage, Bonus: 0},
	}
	for _, tier := range tiers {
		if got := standing.SocialCheck(tier); got != want[tier] {
			t.Errorf("%s: %+v, want %+v", tier, got, want[tier])
		}
	}
	if got := standing.SocialCheck("stranger"); got != (standing.Social{Mode: standing.Straight, Bonus: 0}) {
		t.Errorf("no Standing at all = %+v", got)
	}
	if standing.Advantage != "advantage" || standing.Disadvantage != "disadvantage" || standing.Straight != "" {
		t.Error("modes are misnamed")
	}
}

// A member Shop prices by tier: dearer to those it dislikes, cheaper to its friends.
func TestStandingShapesPrices(t *testing.T) {
	t.Parallel()
	want := map[standing.Tier]int{standing.Hostile: 50, standing.Unfriendly: 20, standing.Neutral: 0, standing.Friendly: -10, standing.Allied: -20}
	for _, tier := range tiers {
		if got := standing.PricePct(tier); got != want[tier] {
			t.Errorf("%s: %d%%, want %d%%", tier, got, want[tier])
		}
	}
	if got := standing.PricePct("stranger"); got != 0 {
		t.Errorf("no Standing at all = %d%%", got)
	}
}

// A member's first reaction starts from the tier.
func TestStandingShapesFirstReactions(t *testing.T) {
	t.Parallel()
	want := map[standing.Tier]standing.Attitude{
		standing.Hostile: standing.AttitudeHostile, standing.Unfriendly: standing.AttitudeIndifferent, standing.Neutral: standing.AttitudeIndifferent,
		standing.Friendly: standing.AttitudeFriendly, standing.Allied: standing.AttitudeFriendly,
	}
	for _, tier := range tiers {
		if got := standing.FirstReaction(tier); got != want[tier] {
			t.Errorf("%s: %s, want %s", tier, got, want[tier])
		}
	}
	if got := standing.FirstReaction("stranger"); got != standing.AttitudeIndifferent {
		t.Errorf("no Standing at all = %s", got)
	}
	if standing.AttitudeHostile != "hostile" || standing.AttitudeIndifferent != "indifferent" || standing.AttitudeFriendly != "friendly" {
		t.Error("attitudes are misnamed")
	}
}

// In a Faction's territory its own entries on an Encounter Table weigh by tier: the party meets its
// patrols more the worse it stands with them, and less the better.
func TestStandingShapesEncounterWeights(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		tier   standing.Tier
		weight int
		want   int
	}{
		{standing.Hostile, 10, 20},
		{standing.Unfriendly, 10, 15},
		{standing.Neutral, 10, 10},
		{standing.Friendly, 10, 5},
		{standing.Allied, 10, 2},
		// A weight never rounds away to nothing, and nothing stays nothing.
		{standing.Allied, 1, 1},
		{standing.Friendly, 1, 1},
		{standing.Allied, 4, 1},
		{standing.Unfriendly, 1, 2},
		{standing.Hostile, 0, 0},
		{standing.Allied, 0, 0},
		{standing.Hostile, 7, 14},
		{standing.Unfriendly, 7, 11},
		{standing.Friendly, 7, 4},
		{standing.Allied, 7, 1},
		{standing.Allied, 8, 2},
		{standing.Tier("stranger"), 7, 7},
	} {
		if got := standing.Weight(c.tier, c.weight); got != c.want {
			t.Errorf("a weight of %d at %s = %d, want %d", c.weight, c.tier, got, c.want)
		}
	}
}

// What a Roll Card says of a Standing: the tier and the Faction, and Advantage or Disadvantage when
// it gives one. A bonus or penalty is carried beside the line as its value.
func TestTheStandingLine(t *testing.T) {
	t.Parallel()
	want := map[standing.Tier]string{
		standing.Hostile:    "Hostile with the Lantern Watch: Disadvantage",
		standing.Unfriendly: "Unfriendly with the Lantern Watch",
		standing.Neutral:    "Neutral with the Lantern Watch",
		standing.Friendly:   "Friendly with the Lantern Watch",
		standing.Allied:     "Allied with the Lantern Watch: Advantage",
	}
	for _, tier := range tiers {
		if got := standing.Line(tier, "the Lantern Watch"); got != want[tier] {
			t.Errorf("%s: %q, want %q", tier, got, want[tier])
		}
	}
	// A long name is cut so the line fits a Roll Card: fifty letters of it at most, whatever they are.
	fifty := strings.Repeat("é", 50)
	if got := standing.Line(standing.Allied, fifty); got != "Allied with "+fifty+": Advantage" {
		t.Errorf("a name of exactly fifty letters: %q", got)
	}
	if got := standing.Line(standing.Hostile, fifty+"x"); got != "Hostile with "+strings.Repeat("é", 49)+"…: Disadvantage" || len([]rune(got)) > 80 {
		t.Errorf("a name of fifty-one letters: %q", got)
	}
}
