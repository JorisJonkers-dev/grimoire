//go:build integration

package live_test

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// Aria (Strength 10) leaps 10 feet with a run and 5 standing, 3 feet up at most, and falls when she
// jumps off a 10-foot ledge.
func jumpTo(ids map[string]string, q, r int) live.Command {
	return live.Command{Kind: live.CmdJump, TokenID: ids["Aria"], Q: q, R: r}
}

func TestAJumpTooFarOntoATakenHexOrByAnotherIsRefused(t *testing.T) {
	t.Parallel()
	_, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 0}}, ElevationFt: 10})
	for want, cmd := range map[string]live.Command{
		"leaps at most 10 feet": jumpTo(ids, 3, 0),
		"jumps at most 3 feet":  jumpTo(ids, 1, 0),
		"Land on a free hex":    jumpTo(ids, 2, 0),
		"not yours to play":     {Kind: live.CmdJump, TokenID: ids["Goblin"], Q: 0, R: 1},
	} {
		t.Run(want, func(t *testing.T) {
			if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
				t.Fatalf("%+v", u)
			}
		})
	}
}

func TestALeapCoversTenFeet(t *testing.T) {
	t.Parallel()
	_, tb, ids := dungeonTable(t)
	if u := tb.playerSays(jumpTo(ids, 1, 1)); token(u.View, "Aria").Q != 1 || token(u.View, "Aria").R != 1 {
		t.Fatalf("Aria = %+v", token(u.View, "Aria"))
	}
}

func TestJumpingOffALedgeFallsAndKnocksProne(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 0}}, ElevationFt: 10})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 0})
	tb.playerSays(jumpTo(ids, 0, 1))
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.dm)
	if !strings.Contains(manuals(v), "Aria falls 10 feet: 1d6 bludgeoning damage.") || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("jumping off the ledge = %s %+v", manuals(v), token(v, "Aria"))
	}
}

// fightingFrom is a fight with Aria standing at q, r.
func fightingFrom(t *testing.T, q, r int) (*table, map[string]string) {
	t.Helper()
	_, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: q, R: r})
	tb.fight(ids, "Aria", "Goblin")
	return tb, ids
}

func TestInAFightAStandingLeapIsHalfAsLong(t *testing.T) {
	t.Parallel()
	tb, ids := fightingFrom(t, 0, 1)
	if u := tb.playerSays(jumpTo(ids, 0, 3)); !strings.Contains(u.Reason, "leaps at most 5 feet") {
		t.Fatalf("a standing leap = %+v", u)
	}
}

func TestInAFightARunningLeapSpendsItsMovement(t *testing.T) {
	t.Parallel()
	tb, ids := fightingFrom(t, 0, 1)
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 3})
	if u := tb.playerSays(jumpTo(ids, 2, 1)); u.View == nil || token(u.View, "Aria").Q != 2 || combatant(u.View, "Aria").MovementFt != 10 {
		t.Fatalf("a running leap = %+v", u)
	}
}

func TestALeapNeedsTheMovementLeft(t *testing.T) {
	t.Parallel()
	tb, ids := fightingFrom(t, 0, 1)
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 3})
	tb.playerSays(jumpTo(ids, 2, 1))
	tb.playerSays(jumpTo(ids, 4, 1))
	if u := tb.playerSays(jumpTo(ids, 3, 1)); !strings.Contains(u.Reason, "too little movement") {
		t.Fatalf("out of movement = %+v", u)
	}
}

// Aria throws the goblin she grapples 5 feet, letting it go Prone and hurt, and hurls a barrel that
// bursts where it lands.
// grappling is a table where Aria grapples the Goblin beside her.
func grappling(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	w, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 1, R: 0})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], SourceID: ids["Aria"], Effect: "grappled"})
	return w, tb, ids
}

func throwGoblin(ids map[string]string, q, r int) live.Command {
	return live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], TargetID: ids["Goblin"], Q: q, R: r}
}

func TestOnlyAGrappledCreatureCanBeThrown(t *testing.T) {
	t.Parallel()
	_, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 1, R: 0})
	if u := tb.playerSays(throwGoblin(ids, 0, 1)); !strings.Contains(u.Reason, "Throw only a creature Aria is grappling") {
		t.Fatalf("%+v", u)
	}
}

func TestACreatureIsThrownAtMostFiveFeetOntoAFreeHex(t *testing.T) {
	t.Parallel()
	_, tb, ids := grappling(t)
	for want, cmd := range map[string]live.Command{"throws at most 5 feet": throwGoblin(ids, 0, 2), "free hex": throwGoblin(ids, 1, 0)} {
		t.Run(want, func(t *testing.T) {
			if u := tb.playerSays(cmd); !strings.Contains(u.Reason, want) {
				t.Fatalf("%+v", u)
			}
		})
	}
}

func TestAThrownCreatureLandsProneHurtAndFree(t *testing.T) {
	t.Parallel()
	w, tb, ids := grappling(t)
	tb.playerSays(throwGoblin(ids, 0, 1))
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.dm)
	g := token(v, "Goblin")
	if g.Q != 0 || g.R != 1 || effect(g, "Grappled") != nil || effect(g, "Prone") == nil || !strings.Contains(manuals(v), "Goblin is thrown and takes 1d6 bludgeoning damage.") {
		t.Fatalf("thrown = %+v %s", g, manuals(v))
	}
}

func TestOnlyAWholeBarrelOrChestIsThrown(t *testing.T) {
	t.Parallel()
	_, tb, ids := dungeonTable(t)
	door, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 2, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 1})
	if u := tb.playerSays(live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], ObjectID: object(door.View, "Door").ID, Q: 1, R: 2}); !strings.Contains(u.Reason, "Only a whole barrel or chest") {
		t.Fatalf("a door = %+v", u)
	}
}

func TestAThrownBarrelBurstsOnWhatItLandsBeside(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 0, R: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "barrel", Q: 1, R: 0, Effect: "prone", RadiusFt: 5})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 1})
	tb.playerSays(live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], ObjectID: object(d.View, "Barrel").ID, Q: 0, R: 2})
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v := look(t, w, tb.dm)
	if b := object(v, "Barrel"); !b.Broken || b.Q != 0 || b.R != 2 || effect(token(v, "Goblin"), "Prone") == nil {
		t.Fatalf("the barrel = %+v, the goblin %+v", b, token(v, "Goblin"))
	}
}
