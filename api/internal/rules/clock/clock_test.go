package clock_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
)

func at(day, hour, minute int) clock.Time { return clock.Time{Day: day, Minute: hour*60 + minute} }

// Time moves on by minutes, over midnight into the next day, and back the same way.
func TestTheClockMovesOn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		from    clock.Time
		minutes int
		want    clock.Time
	}{
		{at(3, 8, 0), 0, at(3, 8, 0)},
		{at(3, 8, 0), 60, at(3, 9, 0)},
		{at(3, 8, 0), 480, at(3, 16, 0)},
		{at(3, 23, 59), 1, at(4, 0, 0)},
		{at(3, 22, 0), 480, at(4, 6, 0)},
		{at(3, 0, 0), 1440, at(4, 0, 0)},
		{at(3, 12, 30), 3 * 1440, at(6, 12, 30)},
		{at(3, 0, 0), 1439, at(3, 23, 59)},
		{at(3, 8, 0), -60, at(3, 7, 0)},
		{at(3, 0, 30), -60, at(2, 23, 30)},
		{at(3, 0, 0), -1, at(2, 23, 59)},
		{at(3, 6, 0), -1440, at(2, 6, 0)},
		// The clock does not run back past the start of the first day.
		{at(0, 0, 30), -60, at(0, 0, 0)},
		{at(1, 0, 0), -5000, at(0, 0, 0)},
	} {
		if got := c.from.Add(c.minutes); got != c.want {
			t.Errorf("%+v and %d minutes = %+v, want %+v", c.from, c.minutes, got, c.want)
		}
	}
}

// Dawn is at six. Every dawn after one time, up to and including another, has passed between them.
func TestCountingDawns(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		from, to clock.Time
		want     int
	}{
		{"no time at all", at(3, 5, 0), at(3, 5, 0), 0},
		{"before dawn to before dawn", at(3, 4, 0), at(3, 5, 59), 0},
		{"up to dawn itself", at(3, 5, 59), at(3, 6, 0), 1},
		{"from dawn on", at(3, 6, 0), at(3, 6, 1), 0},
		{"a morning's march", at(3, 8, 0), at(3, 16, 0), 0},
		{"a night's rest", at(3, 22, 0), at(4, 6, 0), 1},
		{"a night's rest that ends before dawn", at(3, 20, 0), at(4, 4, 0), 0},
		{"midnight to the next midnight", at(3, 0, 0), at(4, 0, 0), 1},
		{"dawn to the next dawn", at(3, 6, 0), at(4, 6, 0), 1},
		{"three days on the road", at(3, 8, 0), at(6, 12, 0), 3},
		{"three days ending before dawn", at(3, 8, 0), at(6, 5, 0), 2},
		{"from before dawn over three days", at(3, 5, 0), at(6, 5, 0), 3},
		{"the clock set back", at(6, 12, 0), at(3, 8, 0), 0},
		{"the first dawn of all", at(0, 0, 0), at(0, 6, 0), 1},
	} {
		if got := clock.Dawns(c.from, c.to); got != c.want {
			t.Errorf("%s: %d dawns, want %d", c.name, got, c.want)
		}
	}
}

// A journey takes its time on the road and a night's camp between its travel days.
func TestHowLongAJourneyTakes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ minutes, days, want int }{
		{0, 0, 0},
		{240, 1, 240},
		{480, 1, 480},
		{500, 2, 500 + 16*60},
		{1080, 3, 1080 + 2*16*60},
		{1, 1, 1},
	} {
		if got := clock.Journey(c.minutes, c.days); got != c.want {
			t.Errorf("%d minutes over %d days = %d, want %d", c.minutes, c.days, got, c.want)
		}
	}
	// A whole day's march and its camp fill a day exactly: three days of it end where they began, three days on.
	if got := at(2, 8, 0).Add(clock.Journey(3*480, 3)); got != at(4, 16, 0) {
		t.Errorf("three days' march from eight = %+v", got)
	}
	if clock.DawnMinute != 360 || clock.ShortRest != 60 || clock.LongRest != 480 {
		t.Errorf("constants %d %d %d", clock.DawnMinute, clock.ShortRest, clock.LongRest)
	}
}

// Whether a time has come: at it or after it.
func TestWhetherATimeHasCome(t *testing.T) {
	t.Parallel()
	now := at(4, 10, 0)
	for _, c := range []struct {
		when clock.Time
		want bool
	}{
		{at(4, 10, 0), true}, {at(4, 9, 59), true}, {at(3, 23, 0), true}, {at(4, 10, 1), false}, {at(5, 0, 0), false}, {at(5, 9, 0), false},
	} {
		if got := now.Reached(c.when); got != c.want {
			t.Errorf("%+v reached at %+v = %v", c.when, now, got)
		}
	}
}
