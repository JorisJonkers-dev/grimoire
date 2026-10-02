package live_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// Polymorph lays a wolf's statistics over Aria with the wolf's hit points as Temporary Hit Points; it
// survives a restart, and when damage takes the last of them Aria reverts and the rest carries over.
func TestPolymorphRevertsWithOverflow(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	for want, cmd := range map[string]live.Command{
		"Choose a creature to become.": {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "polymorph"},
		"No such creature: owl.":       {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "polymorph", MonsterSlug: "owl"},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Reason != want {
			t.Errorf("%s = %+v", want, u)
		}
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "polymorph", MonsterSlug: "wolf"})
	aria := token(d.View, "Aria")
	if aria.Form != "Wolf" || *aria.AC != 13 || aria.TempHP != 11 || *aria.HP != 12 || len(aria.Attacks) != 1 || aria.Attacks[0].Name != "Bite" {
		t.Fatalf("Aria the wolf = %+v", aria)
	}
	if token(p.View, "Aria").Form != "Wolf" {
		t.Fatal("the party sees the form")
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if a := token(look(t, w, tb.dm), "Aria"); a.Form != "Wolf" || *a.AC != 13 || a.TempHP != 11 || a.Attacks[0].Name != "Bite" {
		t.Fatalf("after a restart = %+v", a)
	}
	tb.fight(ids, "Goblin", "Aria")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "shatter", Q: 0, R: 0})
	tb.fill(d.View.Area.DamageRollID, w.dm, 4, 4, 4)
	tb.fill(d.View.Area.Saves[0].RollID, w.player, 1)
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v := look(t, w, tb.dm)
	if a := token(v, "Aria"); a.Form != "" || *a.AC != 16 || a.TempHP != 0 || *a.HP != 11 || a.Attacks[0].Name == "Bite" || effect(a, "Polymorph") != nil {
		t.Fatalf("12 damage: 11 to the wolf, 1 to Aria = %+v", a)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if a := token(look(t, w, tb.dm), "Aria"); a.Form != "" || *a.AC != 16 || a.TempHP != 0 {
		t.Fatalf("reverted after a restart = %+v", a)
	}
}

// Wild Shape takes the Temporary Hit Points it is given, shifting again keeps the true statistics, and
// ending it reverts the druid with the hit points it has.
func TestWildShapeEndsOnDismiss(t *testing.T) {
	t.Parallel()
	_, tb, ids := summonTable(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "wild-shape", MonsterSlug: "wolf", TempHP: 3})
	if a := token(d.View, "Aria"); a.Form != "Wolf" || a.TempHP != 3 {
		t.Fatalf("wild shape = %+v", a)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "wild-shape", MonsterSlug: "hobgoblin"})
	if a := token(d.View, "Aria"); a.Form != "Hobgoblin" || *a.AC != 15 || a.TempHP != 11 {
		t.Fatalf("shifting again = %+v", a)
	}
	for _, e := range token(d.View, "Aria").Effects {
		tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: e.ID})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -2})
	if a := token(d.View, "Aria"); a.Form != "" || *a.AC != 16 || *a.HP != 10 || a.Attacks[0].ToHit != 5 {
		t.Fatalf("back to Aria = %+v", a)
	}
}
