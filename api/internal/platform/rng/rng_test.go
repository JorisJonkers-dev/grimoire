package rng_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
)

func TestSameSeedSameFaces(t *testing.T) {
	t.Parallel()
	a, b := rng.New(42), rng.New(42)
	for range 100 {
		if x, y := a.IntN(20), b.IntN(20); x != y || x < 0 || x >= 20 {
			t.Fatalf("%d vs %d", x, y)
		}
	}
	if first, second := rng.Seed(), rng.Seed(); first == second {
		t.Fatal("two seeds collided")
	}
}
