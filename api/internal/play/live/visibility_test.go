package live_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

func leaks(t *testing.T, u live.Update, word string) bool {
	t.Helper()
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(raw), word)
}

// Fog negative: a Disguised hobgoblin's true identity never reaches the party, on the board or in the
// roster, until the party sees through it; the disguise survives a restart.
func TestDisguisesHoldUntilSeenThrough(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Q: -3})
	ids["Hobgoblin"] = token(d.View, "Hobgoblin").ID
	for want, cmd := range map[string]live.Command{
		"Choose Qualities from":   {Kind: live.CmdSetVisibility, TokenID: ids["Hobgoblin"], Qualities: []string{"blurry"}},
		"can be seen through":     {Kind: live.CmdSetVisibility, TokenID: ids["Hobgoblin"], Qualities: []string{"hidden"}, SeenThrough: []string{"invisible"}},
		"needs the name it shows": {Kind: live.CmdSetVisibility, TokenID: ids["Hobgoblin"], Qualities: []string{"disguised"}},
		"No such token":           {Kind: live.CmdSetVisibility, TokenID: "nope"},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Hobgoblin"], Qualities: []string{"disguised"}, Disguise: " Old Woman "})
	if hb := token(d.View, "Hobgoblin"); hb == nil || hb.Disguise != "Old Woman" || len(hb.Qualities) != 1 || hb.Qualities[0].Quality != "disguised" {
		t.Fatalf("the DM sees the truth = %+v", hb)
	}
	if token(p.View, "Old Woman") == nil || leaks(t, p, "Hobgoblin") || leaks(t, p, "disguised") {
		t.Fatalf("the party sees an old woman = %+v", p.View.Tokens)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.fight(ids, "Aria", "Hobgoblin")
	pv := look(t, w, tb.player)
	if combatant(pv, "Old Woman") == nil || combatant(pv, "Hobgoblin") != nil {
		t.Fatalf("the roster keeps the disguise after a restart = %+v", pv.Combat)
	}
	raw, _ := json.Marshal(pv)
	if strings.Contains(string(raw), "Hobgoblin") {
		t.Fatal("the true name leaks into the party's view")
	}
	_, pu := tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Hobgoblin"], Qualities: []string{"disguised"}, SeenThrough: []string{"disguised"}, Disguise: "Old Woman"})
	if token(pu.View, "Hobgoblin") == nil || combatant(pu.View, "Hobgoblin") == nil {
		t.Fatalf("seen through = %+v", pu.View.Tokens)
	}
}

// An Invisible goblin is not on the party's board until a party member's blindsight reaches it; Faerie
// Fire strips the quality for everyone.
func TestInvisibleAgainstSensesAndReveal(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	_, p := tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"invisible"}})
	if token(p.View, "Goblin") != nil {
		t.Fatal("the party sees an invisible goblin")
	}
	_, p = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "bat", TokenKind: domain.TokenParty, Q: -10})
	if token(p.View, "Goblin") != nil {
		t.Fatal("the bat's blindsight reaches 60 feet, not 60 feet and more")
	}
	bat := token(p.View, "Bat").ID
	_, p = tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: bat, Q: 0, R: 1})
	if token(p.View, "Goblin") == nil {
		t.Fatal("blindsight finds the invisible goblin")
	}
	if _, p = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: bat}); token(p.View, "Goblin") != nil {
		t.Fatal("without the bat the goblin is unseen again")
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], SourceID: ids["Aria"], Effect: "faerie-fire"})
	if token(p.View, "Goblin") == nil || len(token(d.View, "Goblin").Qualities) != 0 {
		t.Fatalf("faerie fire outlines the goblin = %+v", token(d.View, "Goblin"))
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if g := token(look(t, w, tb.dm), "Goblin"); len(g.Qualities) != 0 {
		t.Fatalf("the outline survives a restart = %+v", g)
	}
}

// A Reveal in an area strips its Qualities from everything there, a disguise with them.
func TestRevealsInAnArea(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"invisible", "disguised", "hidden"}, Disguise: "Rock"})
	tb.fight(ids, "Aria", "Goblin")
	tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "outlining-burst", Q: 3, R: 0})
	d := look(t, w, tb.dm)
	tb.fill(d.Area.DamageRollID, w.player, 1)
	tb.fill(d.Area.Saves[0].RollID, w.dm, 20)
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	g := token(look(t, w, tb.dm), "Goblin")
	if len(g.Qualities) != 1 || g.Qualities[0].Quality != "hidden" || g.Disguise != "" {
		t.Fatalf("only hidden is left = %+v", g)
	}
}
