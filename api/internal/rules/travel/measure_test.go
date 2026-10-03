package travel_test

import (
	"math"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/travel"
)

// A route is measured hex by hex through its waypoints, and in miles by the Map's scale.
func TestMeasuringARouteOnACalibratedGrid(t *testing.T) {
	t.Parallel()
	at := func(q, r int) hex.Coord { return hex.Coord{Q: q, R: r} }
	for _, c := range []struct {
		name      string
		waypoints []hex.Coord
		scale     float64
		hexes     int
		miles     float64
	}{
		{"no route", nil, 6, 0, 0},
		{"one point", []hex.Coord{at(2, 1)}, 6, 0, 0},
		{"the same point twice", []hex.Coord{at(2, 1), at(2, 1)}, 6, 0, 0},
		{"a neighbour", []hex.Coord{at(0, 0), at(1, 0)}, 6, 1, 6},
		{"five hexes along a row", []hex.Coord{at(0, 0), at(5, 0)}, 6, 5, 30},
		{"the same row at a mile to the hex", []hex.Coord{at(0, 0), at(5, 0)}, 1, 5, 5},
		{"on a fine grid", []hex.Coord{at(0, 0), at(5, 0)}, 0.5, 5, 2.5},
		{"on a coarse grid", []hex.Coord{at(0, 0), at(5, 0)}, 24, 5, 120},
		{"across rows", []hex.Coord{at(0, 0), at(-2, 5)}, 6, 5, 30},
		{"around a corner", []hex.Coord{at(0, 0), at(5, 0), at(5, 3)}, 6, 8, 48},
		{"there and back", []hex.Coord{at(0, 0), at(4, -2), at(0, 0)}, 6, 8, 48},
		{"a detour is longer than the straight way", []hex.Coord{at(0, 0), at(0, 4), at(4, 0)}, 6, 8, 48},
	} {
		got := travel.Measure(c.waypoints, c.scale)
		if got.Hexes != c.hexes || math.Abs(got.Miles-c.miles) > 1e-9 {
			t.Errorf("%s: %d hexes, %v miles; want %d, %v", c.name, got.Hexes, got.Miles, c.hexes, c.miles)
		}
	}
}

// Time on the road is the miles at the pace, in whole minutes rounded up, over travel days of eight hours.
func TestTimingMeasuredMiles(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		miles   float64
		pace    travel.Pace
		minutes int
		days    int
	}{
		{0, travel.Normal, 0, 0},
		{30, travel.Normal, 600, 2},
		{30, travel.Slow, 900, 2},
		{30, travel.Fast, 450, 1},
		{24, travel.Normal, 480, 1},
		{24.1, travel.Normal, 482, 2},
		{2.5, travel.Normal, 50, 1},
		{0.1, travel.Fast, 2, 1},
		{0.5, travel.Slow, 15, 1},
		{96, travel.Fast, 1440, 3},
		{96.01, travel.Fast, 1441, 4},
	} {
		minutes, days := travel.Time(c.miles, c.pace)
		if minutes != c.minutes || days != c.days {
			t.Errorf("%v miles at a %s pace = %d min over %d days; want %d over %d", c.miles, c.pace, minutes, days, c.minutes, c.days)
		}
	}
	// Whole miles take as long measured as they do along a route.
	for _, miles := range []int{1, 12, 25, 100} {
		for _, p := range []travel.Pace{travel.Slow, travel.Normal, travel.Fast} {
			leg := travel.Plan(miles, p)
			if minutes, days := travel.Time(float64(miles), p); minutes != leg.Minutes || days != leg.Days {
				t.Errorf("%d miles at a %s pace: measured %d min %d days, a route %d min %d days", miles, p, minutes, days, leg.Minutes, leg.Days)
			}
		}
	}
}
