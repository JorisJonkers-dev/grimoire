package movement_test

import (
	"fmt"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/movement"
)

func TestAFallDealsAD6PerTenFeetUpToTwentyAndKnocksProne(t *testing.T) {
	t.Parallel()
	for feet, want := range map[int]string{0: "", 9: "", 10: "1d6", 25: "2d6", 200: "20d6", 500: "20d6", -10: ""} {
		dice, prone := movement.Fall(feet)
		if dice != want || prone != (want != "") {
			t.Errorf("a %d ft fall = %q %v", feet, dice, prone)
		}
	}
}

func TestALongJumpCoversStrengthInFeetHalvedWithoutARunUp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		str     int
		running bool
		want    int
	}{{15, true, 15}, {15, false, 7}, {-2, true, 0}, {-2, false, 0}} {
		t.Run(fmt.Sprint(c.str, c.running), func(t *testing.T) {
			if got := movement.LongJumpFt(c.str, c.running); got != c.want {
				t.Fatalf("long jump = %d", got)
			}
		})
	}
}

func TestAHighJumpIsThreePlusTheModifierHalvedWithoutARunUp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mod     int
		running bool
		want    int
	}{{2, true, 5}, {2, false, 2}, {-5, true, 0}, {-1, false, 1}} {
		t.Run(fmt.Sprint(c.mod, c.running), func(t *testing.T) {
			if got := movement.HighJumpFt(c.mod, c.running); got != c.want {
				t.Fatalf("high jump = %d", got)
			}
		})
	}
}

func TestARunUpIsTenFeet(t *testing.T) {
	t.Parallel()
	if !movement.Running(10) || movement.Running(5) {
		t.Fatal("run-up")
	}
}

func TestAThrowReachesFiveFeetPerStrengthModifierAtLeastFive(t *testing.T) {
	t.Parallel()
	for mod, want := range map[int]int{-1: 5, 0: 5, 1: 5, 3: 15} {
		t.Run(fmt.Sprint(mod), func(t *testing.T) {
			if got := movement.ThrowRangeFt(mod); got != want {
				t.Fatalf("range = %d", got)
			}
		})
	}
}

func TestAThrownCreatureTakesAD6AndAFallAtMostTwentyDice(t *testing.T) {
	t.Parallel()
	if movement.ThrownDice != "1d6" || movement.FallMaxDice != 20 {
		t.Fatal("constants")
	}
}
