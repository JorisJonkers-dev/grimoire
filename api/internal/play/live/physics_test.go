package live_test

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// Aria (Strength 10) leaps 10 feet with a run and 5 standing, 3 feet up at most, and falls when she
// jumps off a 10-foot ledge.
func TestJumping(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	jump := func(q, r int) live.Command { return live.Command{Kind: live.CmdJump, TokenID: ids["Aria"], Q: q, R: r} }
	tb.dmSays(live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 0}}, ElevationFt: 10})
	for want, cmd := range map[string]live.Command{
		"leaps at most 10 feet": jump(3, 0),
		"jumps at most 3 feet":  jump(1, 0),
		"Land on a free hex":    jump(2, 0),
		"not yours to play":     {Kind: live.CmdJump, TokenID: ids["Goblin"], Q: 0, R: 1},
	} {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(jump(1, 1)); token(u.View, "Aria").Q != 1 || token(u.View, "Aria").R != 1 {
		t.Fatalf("a 10-foot leap = %+v", token(u.View, "Aria"))
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 0})
	tb.playerSays(jump(0, 1))
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.dm)
	if !strings.Contains(manuals(v), "Aria falls 10 feet: 1d6 bludgeoning damage.") || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("jumping off the ledge = %s %+v", manuals(v), token(v, "Aria"))
	}
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(v, "Aria"), "Prone").ID})
	tb.fight(ids, "Aria", "Goblin")
	if u := tb.playerSays(jump(0, 3)); !strings.Contains(u.Reason, "leaps at most 5 feet") {
		t.Fatalf("a standing leap = %+v", u)
	}
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 3})
	if u := tb.playerSays(jump(2, 1)); u.View == nil || token(u.View, "Aria").Q != 2 || combatant(u.View, "Aria").MovementFt != 10 {
		t.Fatalf("a running leap spends 10 feet = %+v", u)
	}
	tb.playerSays(jump(4, 1))
	if u := tb.playerSays(jump(3, 1)); !strings.Contains(u.Reason, "too little movement") {
		t.Fatalf("out of movement = %+v", u)
	}
}

// Aria throws the goblin she grapples 5 feet, letting it go Prone and hurt, and hurls a barrel that
// bursts where it lands.
func TestThrowing(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 1, R: 0})
	throw := live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], TargetID: ids["Goblin"], Q: 0, R: 1}
	if u := tb.playerSays(throw); !strings.Contains(u.Reason, "Throw only a creature Aria is grappling") {
		t.Fatalf("not grappling = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], SourceID: ids["Aria"], Effect: "grappled"})
	far := throw
	far.Q, far.R = 0, 2
	for want, cmd := range map[string]live.Command{"throws at most 5 feet": far, "free hex": {Kind: live.CmdThrow, TokenID: ids["Aria"], TargetID: ids["Goblin"], Q: 1, R: 0}} {
		if u := tb.playerSays(cmd); !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	tb.playerSays(throw)
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.dm)
	g := token(v, "Goblin")
	if g.Q != 0 || g.R != 1 || effect(g, "Grappled") != nil || effect(g, "Prone") == nil || !strings.Contains(manuals(v), "Goblin is thrown and takes 1d6 bludgeoning damage.") {
		t.Fatalf("thrown = %+v %s", g, manuals(v))
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "barrel", Q: 1, R: 0, Effect: "prone", RadiusFt: 5})
	barrel := object(d.View, "Barrel").ID
	door, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 2, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(g, "Prone").ID})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 1})
	if u := tb.playerSays(live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], ObjectID: object(door.View, "Door").ID, Q: 1, R: 2}); !strings.Contains(u.Reason, "Only a whole barrel or chest") {
		t.Fatalf("a door = %+v", u)
	}
	tb.playerSays(live.Command{Kind: live.CmdThrow, TokenID: ids["Aria"], ObjectID: barrel, Q: 0, R: 2})
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v = look(t, w, tb.dm)
	if b := object(v, "Barrel"); !b.Broken || b.Q != 0 || b.R != 2 || effect(token(v, "Goblin"), "Prone") == nil {
		t.Fatalf("the barrel bursts on the goblin = %+v %+v", b, token(v, "Goblin"))
	}
}
