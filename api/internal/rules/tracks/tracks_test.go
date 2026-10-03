package tracks_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tracks"
)

// A Track's score stays within its bounds.
func TestAScoreStaysWithinItsTrack(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ lo, hi, score, want int }{
		{0, 10, 5, 5}, {0, 10, 0, 0}, {0, 10, 10, 10}, {0, 10, -1, 0}, {0, 10, 11, 10}, {-5, 5, -9, -5}, {-5, 5, 9, 5}, {3, 3, 7, 3},
	} {
		if got := tracks.Clamp(c.lo, c.hi, c.score); got != c.want {
			t.Errorf("%d within %d to %d = %d, want %d", c.score, c.lo, c.hi, got, c.want)
		}
	}
}

// A threshold is crossed when the score reaches or passes it going its way: a rising one on the way up,
// a falling one on the way down. A score that starts on a threshold has already crossed it.
func TestCrossingThresholds(t *testing.T) {
	t.Parallel()
	// Broken at 8 rising, calm again at 2 falling, shaken at 4 rising.
	const broken, calm, shaken = 0, 1, 2
	all := []tracks.Threshold{{At: 8, Rising: true}, {At: 2, Rising: false}, {At: 4, Rising: true}}
	for _, c := range []struct {
		name     string
		from, to int
		want     []int
	}{
		{"no change", 5, 5, nil},
		{"up, short of the first", 0, 3, nil},
		{"up, onto the first", 0, 4, []int{shaken}},
		{"up, past the first", 3, 5, []int{shaken}},
		{"up, from the first", 4, 7, nil},
		{"up, past both, nearest first", 0, 9, []int{shaken, broken}},
		{"up, onto the second from the first", 4, 8, []int{broken}},
		{"down, short of the falling one", 9, 3, nil},
		{"down, onto the falling one", 9, 2, []int{calm}},
		{"down, past the falling one", 3, 0, []int{calm}},
		{"down, from the falling one", 2, 0, nil},
	} {
		if got := tracks.Crossed(all, c.from, c.to); !slices.Equal(got, c.want) {
			t.Errorf("%s (%d to %d) = %v", c.name, c.from, c.to, got)
		}
	}
	// On the way down the nearest is still first.
	if got := tracks.Crossed([]tracks.Threshold{{At: 1, Rising: false}, {At: 2, Rising: false}}, 5, 0); !slices.Equal(got, []int{1, 0}) {
		t.Errorf("down past two = %v", got)
	}
	// Two at one score keep the order they were given in.
	if got := tracks.Crossed([]tracks.Threshold{{At: 4, Rising: true}, {At: 4, Rising: true}}, 0, 4); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("two at one score = %v", got)
	}
	if got := tracks.Crossed(nil, 0, 10); got != nil {
		t.Errorf("no thresholds = %v", got)
	}
}
