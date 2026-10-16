package combat_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
)

func TestATurnStartsWithEveryActionAndFullMovement(t *testing.T) {
	t.Parallel()
	if e := combat.Fresh(30); e != (combat.Economy{Action: true, BonusAction: true, Reaction: true, MovementFt: 30, Interaction: true}) {
		t.Fatalf("fresh = %+v", e)
	}
}

func TestEachActionIsSpentOnce(t *testing.T) {
	t.Parallel()
	for _, r := range []combat.Resource{combat.Action, combat.BonusAction, combat.Reaction} {
		t.Run(fmt.Sprint(r), func(t *testing.T) {
			e, ok := combat.Fresh(30).Spend(r)
			if !ok {
				t.Fatal("first spend refused")
			}
			if _, ok = e.Spend(r); ok {
				t.Fatal("second spend allowed")
			}
		})
	}
}

func TestSpendingTheActionsLeavesMovementAndTheInteraction(t *testing.T) {
	t.Parallel()
	e := combat.Fresh(30)
	for _, r := range []combat.Resource{combat.Action, combat.BonusAction, combat.Reaction} {
		e, _ = e.Spend(r)
	}
	if e != (combat.Economy{MovementFt: 30, Interaction: true}) {
		t.Fatalf("spent = %+v", e)
	}
}

func TestAnUnknownResourceCannotBeSpent(t *testing.T) {
	t.Parallel()
	if _, ok := combat.Fresh(30).Spend(combat.Resource(9)); ok {
		t.Fatal("spent")
	}
}

func TestMovementIsSpentUpToWhatIsLeft(t *testing.T) {
	t.Parallel()
	e := combat.Fresh(30)
	if moved, ok := e.Move(30); !ok || moved.MovementFt != 0 {
		t.Fatalf("move all = %+v %v", moved, ok)
	}
	if left, ok := e.Move(0); !ok || left.MovementFt != 30 {
		t.Fatalf("move nothing = %+v %v", left, ok)
	}
}

func TestMovingTooFarOrBackwardsIsRefused(t *testing.T) {
	t.Parallel()
	for _, ft := range []int{31, -5} {
		t.Run(fmt.Sprint(ft), func(t *testing.T) {
			if left, ok := combat.Fresh(30).Move(ft); ok || left.MovementFt != 30 {
				t.Fatalf("move = %+v %v", left, ok)
			}
		})
	}
}

var initiatives = []int{12, 18, 12, 7, 18, 3}

func TestInitiativeCountsRunHighestFirstWithoutRepeats(t *testing.T) {
	t.Parallel()
	if got := combat.Counts(initiatives); !reflect.DeepEqual(got, []int{18, 12, 7, 3}) {
		t.Fatalf("counts = %v", got)
	}
}

func TestNoInitiativesMakeNoCounts(t *testing.T) {
	t.Parallel()
	if combat.Counts(nil) != nil {
		t.Fatal("counts")
	}
}

func TestAnInitiativeRanksByTheCountsAboveIt(t *testing.T) {
	t.Parallel()
	for total, rank := range map[int]int{18: 1, 12: 2, 7: 3, 3: 4, 20: 1, 1: 5} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			if got := combat.Rank(initiatives, total); got != rank {
				t.Fatalf("rank = %d, want %d", got, rank)
			}
		})
	}
}

func TestTheNextCountFollowsAndWrapsIntoANewRound(t *testing.T) {
	t.Parallel()
	for _, s := range []struct {
		current, next int
		round         bool
	}{{18, 12, false}, {12, 7, false}, {7, 3, false}, {3, 18, true}, {99, 18, false}, {13, 12, false}, {1, 18, true}} {
		t.Run(fmt.Sprint(s.current), func(t *testing.T) {
			if n, r := combat.Next(initiatives, s.current); n != s.next || r != s.round {
				t.Fatalf("next = %d %v, want %d %v", n, r, s.next, s.round)
			}
		})
	}
}

func TestWithoutInitiativesTheCountStays(t *testing.T) {
	t.Parallel()
	if n, r := combat.Next(nil, 5); n != 5 || r {
		t.Fatalf("next = %d %v", n, r)
	}
}

func TestTheAttackActionsAttacks(t *testing.T) {
	t.Parallel()
	e := combat.Fresh(30)
	if !e.CanAttack() || !e.Interaction || e.AttacksLeft != 0 {
		t.Fatalf("fresh = %+v", e)
	}
	e, ok := e.Attack(2, false)
	if !ok || e.Action || e.AttacksLeft != 1 || e.LightAttack || !e.CanAttack() {
		t.Fatalf("the first of two attacks = %+v", e)
	}
	if e.CanOffHand(false) {
		t.Fatal("no off-hand attack before a Light weapon attack")
	}
	e, ok = e.Attack(2, true)
	if !ok || e.AttacksLeft != 0 || !e.LightAttack || e.CanAttack() {
		t.Fatalf("the second attack, with a Light weapon = %+v", e)
	}
	if _, ok := e.Attack(2, false); ok {
		t.Fatal("no third attack")
	}
	if !e.CanOffHand(false) || !e.CanOffHand(true) {
		t.Fatal("the off-hand attack opens")
	}
	paid, ok := e.OffHandAttack(false)
	if !ok || paid.BonusAction || !paid.OffHand || paid.CanOffHand(true) {
		t.Fatalf("the off-hand attack costs the bonus action = %+v", paid)
	}
	if _, ok := paid.OffHandAttack(true); ok {
		t.Fatal("one off-hand attack a turn")
	}
	nick, ok := e.OffHandAttack(true)
	if !ok || !nick.BonusAction || !nick.OffHand {
		t.Fatalf("Nick folds it into the Attack action = %+v", nick)
	}
	spent := e
	spent.BonusAction = false
	if spent.CanOffHand(false) || !spent.CanOffHand(true) {
		t.Fatal("without a bonus action only Nick makes it")
	}
	one, ok := combat.Fresh(30).Attack(0, false)
	if !ok || one.AttacksLeft != 0 {
		t.Fatalf("one attack when nothing says more = %+v", one)
	}
	used, ok := combat.Fresh(30).Interact()
	if !ok || used.Interaction {
		t.Fatalf("interact = %+v", used)
	}
	if _, ok := used.Interact(); ok {
		t.Fatal("one free interaction a turn")
	}
}

func TestSwappingWeaponsFollowsTheEquipRules(t *testing.T) {
	t.Parallel()
	fresh := combat.Fresh(30)
	if e, ok := fresh.Swap(1, false); !ok || e.Interaction || !e.Action {
		t.Fatalf("one weapon before attacking takes the free interaction = %+v %v", e, ok)
	}
	if e, ok := fresh.Swap(2, false); !ok || e.Action || !e.Interaction {
		t.Fatalf("two weapons before attacking take the Utilize action = %+v %v", e, ok)
	}
	attacked, _ := fresh.Attack(2, false)
	if attacked.Equips != 1 {
		t.Fatalf("an attack brings one equip = %+v", attacked)
	}
	if e, ok := attacked.Swap(2, false); !ok || e.Equips != 0 || e.Interaction {
		t.Fatalf("an attack's equip and the free interaction cover two = %+v %v", e, ok)
	}
	if e, ok := attacked.Swap(1, false); !ok || e.Equips != 0 || !e.Interaction {
		t.Fatalf("an attack's equip covers one = %+v %v", e, ok)
	}
	if _, ok := attacked.Swap(3, false); ok {
		t.Fatal("three changes with the action spent")
	}
	if e, ok := fresh.Swap(1, true); !ok || e.Action {
		t.Fatalf("a shield takes the Utilize action = %+v %v", e, ok)
	}
	if _, ok := attacked.Swap(1, true); ok {
		t.Fatal("a shield without the action")
	}
	twice, _ := attacked.Attack(2, false)
	if twice.Equips != 2 {
		t.Fatalf("each attack brings an equip = %+v", twice)
	}
}
