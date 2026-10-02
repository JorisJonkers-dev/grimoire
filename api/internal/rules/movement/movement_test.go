package movement_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/movement"
)

func TestFalling(t *testing.T) {
	t.Parallel()
	for feet, want := range map[int]string{0: "", 9: "", 10: "1d6", 25: "2d6", 200: "20d6", 500: "20d6", -10: ""} {
		dice, prone := movement.Fall(feet)
		if dice != want || prone != (want != "") {
			t.Errorf("a %d ft fall = %q %v", feet, dice, prone)
		}
	}
}

func TestJumping(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		str     int
		running bool
		want    int
	}{{15, true, 15}, {15, false, 7}, {-2, true, 0}, {-2, false, 0}} {
		if got := movement.LongJumpFt(c.str, c.running); got != c.want {
			t.Errorf("long jump %d %v = %d", c.str, c.running, got)
		}
	}
	for _, c := range []struct {
		mod     int
		running bool
		want    int
	}{{2, true, 5}, {2, false, 2}, {-5, true, 0}, {-1, false, 1}} {
		if got := movement.HighJumpFt(c.mod, c.running); got != c.want {
			t.Errorf("high jump %d %v = %d", c.mod, c.running, got)
		}
	}
	if !movement.Running(10) || movement.Running(5) {
		t.Error("a run-up is 10 feet")
	}
}

func TestThrowing(t *testing.T) {
	t.Parallel()
	for mod, want := range map[int]int{-1: 5, 0: 5, 1: 5, 3: 15} {
		if got := movement.ThrowRangeFt(mod); got != want {
			t.Errorf("throw with %d = %d", mod, got)
		}
	}
	if movement.ThrownDice != "1d6" || movement.FallMaxDice != 20 {
		t.Error("constants")
	}
}
