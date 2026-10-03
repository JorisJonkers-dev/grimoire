package standing_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
)

// Swaying a creature is a check against 15, or against its Intelligence when that is higher.
func TestTheInfluenceDC(t *testing.T) {
	t.Parallel()
	for intelligence, want := range map[int]int{0: 15, 3: 15, 10: 15, 15: 15, 16: 16, 22: 22} {
		if got := standing.InfluenceDC(intelligence); got != want {
			t.Errorf("Intelligence %d: DC %d, want %d", intelligence, got, want)
		}
	}
}

// A creature's attitude is itself a source: Advantage with a Friendly one, Disadvantage with a Hostile.
func TestAttitudeIsAnAdvantageSource(t *testing.T) {
	t.Parallel()
	want := map[standing.Attitude]standing.Mode{
		standing.AttitudeFriendly: standing.Advantage, standing.AttitudeIndifferent: standing.Straight, standing.AttitudeHostile: standing.Disadvantage, "": standing.Straight,
	}
	for attitude, mode := range want {
		if got := standing.AttitudeMode(attitude); got != mode {
			t.Errorf("%q: %q, want %q", attitude, got, mode)
		}
	}
}

// A check that meets the DC moves the attitude a step towards Friendly; one that misses by five or
// more moves it a step towards Hostile; a near miss changes nothing. It never moves past either end.
func TestInfluenceMovesAnAttitude(t *testing.T) {
	t.Parallel()
	const dc = 15
	for _, c := range []struct {
		from  standing.Attitude
		total int
		want  standing.Attitude
	}{
		{standing.AttitudeIndifferent, 15, standing.AttitudeFriendly},
		{standing.AttitudeIndifferent, 22, standing.AttitudeFriendly},
		{standing.AttitudeHostile, 15, standing.AttitudeIndifferent},
		{standing.AttitudeFriendly, 30, standing.AttitudeFriendly},
		{standing.AttitudeIndifferent, 14, standing.AttitudeIndifferent},
		{standing.AttitudeIndifferent, 11, standing.AttitudeIndifferent},
		{standing.AttitudeIndifferent, 10, standing.AttitudeHostile},
		{standing.AttitudeFriendly, 10, standing.AttitudeIndifferent},
		{standing.AttitudeFriendly, 3, standing.AttitudeIndifferent},
		{standing.AttitudeHostile, 1, standing.AttitudeHostile},
		{standing.AttitudeHostile, 14, standing.AttitudeHostile},
		// An attitude nobody set is Indifferent.
		{"", 15, standing.AttitudeFriendly},
		{"", 10, standing.AttitudeHostile},
		{"", 12, standing.AttitudeIndifferent},
	} {
		if got := standing.Sway(c.from, c.total, dc); got != c.want {
			t.Errorf("%q swayed with %d against %d = %q, want %q", c.from, c.total, dc, got, c.want)
		}
	}
}
