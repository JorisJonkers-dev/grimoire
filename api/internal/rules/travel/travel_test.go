package travel_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/travel"
)

func TestPacesHaveNamesAndSpeeds(t *testing.T) {
	for name, want := range map[string]struct {
		pace travel.Pace
		mph  int
	}{"slow": {travel.Slow, 2}, "normal": {travel.Normal, 3}, "fast": {travel.Fast, 4}} {
		p, ok := travel.ParsePace(name)
		if !ok || p != want.pace || p.String() != name || p.MilesPerHour() != want.mph {
			t.Errorf("%s = %v %v %s %d", name, p, ok, p, p.MilesPerHour())
		}
	}
	if p, ok := travel.ParsePace("gallop"); ok || p != travel.Normal {
		t.Errorf("gallop = %v %v", p, ok)
	}
}

func TestPlanTimesALegAndCountsItsDays(t *testing.T) {
	for _, c := range []struct {
		mi      int
		pace    travel.Pace
		minutes int
		days    int
	}{
		{0, travel.Normal, 0, 0},
		{1, travel.Slow, 30, 1},
		{12, travel.Normal, 240, 1},
		{24, travel.Normal, 480, 1},
		{25, travel.Normal, 500, 2},
		{30, travel.Fast, 450, 1},
		{32, travel.Fast, 480, 1},
		{33, travel.Fast, 495, 2},
		{50, travel.Slow, 1500, 4},
	} {
		got := travel.Plan(c.mi, c.pace)
		want := travel.Leg{DistanceMi: c.mi, Pace: c.pace, Minutes: c.minutes, Days: c.days}
		if got != want {
			t.Errorf("Plan(%d, %s) = %+v, want %+v", c.mi, c.pace, got, want)
		}
	}
	if travel.HoursPerDay != 8 || travel.SightHexes != 2 {
		t.Errorf("constants %d %d", travel.HoursPerDay, travel.SightHexes)
	}
}
