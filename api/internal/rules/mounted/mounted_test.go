package mounted_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/mounted"
)

// Mounting and dismounting each cost half the rider's Speed.
func TestMountingCostsHalfTheRidersSpeed(t *testing.T) {
	t.Parallel()
	for speed, want := range map[int]int{30: 15, 25: 10, 40: 20, 5: 5, 0: 0} {
		if got := mounted.Cost(speed); got != want {
			t.Errorf("Speed %d: %d ft, want %d", speed, got, want)
		}
	}
}

// A rider keeps its seat on a Dexterity save of 10 or more.
func TestKeepingTheSaddle(t *testing.T) {
	t.Parallel()
	if mounted.FallDC != 10 {
		t.Errorf("the save is DC %d", mounted.FallDC)
	}
	for total, want := range map[int]bool{-2: true, 9: true, 10: false, 11: false, 25: false} {
		if got := mounted.Falls(total); got != want {
			t.Errorf("a save of %d falls: %v", total, got)
		}
	}
}

// A controlled mount only Dashes, Disengages or Dodges; an independent one acts as it likes.
func TestWhatAControlledMountDoes(t *testing.T) {
	t.Parallel()
	for action, want := range map[actions.Action]bool{
		actions.Dash: true, actions.Disengage: true, actions.Dodge: true, actions.Hide: false, actions.Help: false, actions.Utilize: false, actions.Influence: false, "": false,
	} {
		if got := mounted.Allows(true, action); got != want {
			t.Errorf("a controlled mount taking %q: %v", action, got)
		}
		if !mounted.Allows(false, action) {
			t.Errorf("an independent mount taking %q is refused", action)
		}
	}
}
