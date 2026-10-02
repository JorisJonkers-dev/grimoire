package objects_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/objects"
)

func TestWhatObjectsBlock(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		state       objects.State
		sight, move bool
	}{
		{objects.State{Kind: objects.Door}, true, true},
		{objects.State{Kind: objects.Door, Open: true}, false, false},
		{objects.State{Kind: objects.Door, Broken: true}, false, false},
		{objects.State{Kind: objects.Curtain}, true, false},
		{objects.State{Kind: objects.Curtain, Open: true}, false, false},
		{objects.State{Kind: objects.Chest, Open: true}, false, true},
		{objects.State{Kind: objects.Barrel}, false, true},
		{objects.State{Kind: objects.Barrel, Broken: true}, false, false},
		{objects.State{Kind: objects.Destructible, Open: true}, true, true},
		{objects.State{Kind: objects.Lever}, false, false},
		{objects.State{Kind: "statue"}, false, false},
	} {
		if sight, move := c.state.Blocks(); sight != c.sight || move != c.move {
			t.Errorf("%+v blocks sight %v, movement %v", c.state, sight, move)
		}
	}
}

func TestUsingAndBreaking(t *testing.T) {
	t.Parallel()
	for k, usable := range map[objects.Kind]bool{objects.Door: true, objects.Curtain: true, objects.Chest: true, objects.Lever: true, objects.Barrel: false, objects.Destructible: false} {
		if got := (objects.State{Kind: k}).Usable(); got != usable {
			t.Errorf("%s usable = %v", k, got)
		}
		if (objects.State{Kind: k, Broken: true}).Usable() {
			t.Errorf("a broken %s is usable", k)
		}
		if !objects.Valid(k) {
			t.Errorf("%s is a kind", k)
		}
	}
	if objects.Valid("statue") || len(objects.Kinds()) != 6 {
		t.Error("kinds")
	}
	for k, want := range map[objects.Kind][2]int{
		objects.Door: {15, 18}, objects.Destructible: {15, 18}, objects.Chest: {15, 13}, objects.Barrel: {15, 9}, objects.Curtain: {11, 2}, objects.Lever: {19, 5},
	} {
		if ac, hp := objects.Defaults(k); ac != want[0] || hp != want[1] {
			t.Errorf("%s defaults %d %d", k, ac, hp)
		}
	}
	for _, c := range []struct {
		hp, amount, left int
		broken           bool
	}{{18, 5, 13, false}, {5, 5, 0, true}, {5, 9, 0, true}, {5, -3, 5, false}, {1, 0, 1, false}} {
		if left, broken := objects.Hit(c.hp, c.amount); left != c.left || broken != c.broken {
			t.Errorf("hit %d for %d = %d %v", c.hp, c.amount, left, broken)
		}
	}
}
