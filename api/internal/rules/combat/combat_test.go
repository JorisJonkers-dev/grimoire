package combat_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
)

func TestEconomy(t *testing.T) {
	t.Parallel()
	e := combat.Fresh(30)
	if e != (combat.Economy{Action: true, BonusAction: true, Reaction: true, MovementFt: 30}) {
		t.Fatalf("fresh = %+v", e)
	}
	for _, r := range []combat.Resource{combat.Action, combat.BonusAction, combat.Reaction} {
		var ok bool
		if e, ok = e.Spend(r); !ok {
			t.Fatalf("first spend of %d refused", r)
		}
		if _, ok = e.Spend(r); ok {
			t.Fatalf("second spend of %d allowed", r)
		}
	}
	if e != (combat.Economy{MovementFt: 30}) {
		t.Fatalf("spent = %+v", e)
	}
	if _, ok := e.Spend(combat.Resource(9)); ok {
		t.Fatal("unknown resource spent")
	}
	moved, ok := e.Move(30)
	if !ok || moved.MovementFt != 0 {
		t.Fatalf("move all = %+v %v", moved, ok)
	}
	if left, ok := e.Move(31); ok || left.MovementFt != 30 {
		t.Fatalf("move too far = %+v %v", left, ok)
	}
	if left, ok := e.Move(-5); ok || left.MovementFt != 30 {
		t.Fatalf("move backwards = %+v %v", left, ok)
	}
	if left, ok := e.Move(0); !ok || left.MovementFt != 30 {
		t.Fatalf("move nothing = %+v %v", left, ok)
	}
}

func TestInitiative(t *testing.T) {
	t.Parallel()
	totals := []int{12, 18, 12, 7, 18, 3}
	if got := combat.Counts(totals); !reflect.DeepEqual(got, []int{18, 12, 7, 3}) {
		t.Fatalf("counts = %v", got)
	}
	for total, rank := range map[int]int{18: 1, 12: 2, 7: 3, 3: 4, 20: 1, 1: 5} {
		if got := combat.Rank(totals, total); got != rank {
			t.Errorf("rank of %d = %d, want %d", total, got, rank)
		}
	}
	steps := []struct {
		current, next int
		round         bool
	}{{18, 12, false}, {12, 7, false}, {7, 3, false}, {3, 18, true}, {99, 18, false}, {13, 12, false}, {1, 18, true}}
	for _, s := range steps {
		if n, r := combat.Next(totals, s.current); n != s.next || r != s.round {
			t.Errorf("after %d = %d %v, want %d %v", s.current, n, r, s.next, s.round)
		}
	}
	if n, r := combat.Next(nil, 5); n != 5 || r {
		t.Fatalf("empty = %d %v", n, r)
	}
	if combat.Counts(nil) != nil {
		t.Fatal("no totals, no counts")
	}
}
