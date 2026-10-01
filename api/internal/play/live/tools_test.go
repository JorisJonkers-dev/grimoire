package live_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

func tokenNamed(t *testing.T, v *live.View, label string) live.TokenView {
	t.Helper()
	for _, tv := range v.Tokens {
		if tv.Label == label {
			return tv
		}
	}
	t.Fatalf("no token %s in %+v", label, v.Tokens)
	return live.TokenView{}
}

func TestTheDMsToolsSpawnAdjustAndUndoByTheActionLog(t *testing.T) {
	t.Parallel()
	w, tb := ambushTable(t)
	m := w.dungeon(t)
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != want {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	spawn := func(q, r int, monsters ...live.SpawnMonster) live.Command {
		return live.Command{Kind: live.CmdSpawnEncounter, Q: q, R: r, Monsters: monsters}
	}
	refuse("Spawn between 1 and 30 creatures.", spawn(0, 0))
	refuse("Spawn between 1 and 30 creatures.", spawn(0, 0, live.SpawnMonster{Slug: "goblin", Count: 31}))
	refuse("Spawn at least one of each creature.", spawn(0, 0, live.SpawnMonster{Slug: "goblin"}))
	refuse("That hex is off the map.", spawn(90, 0, live.SpawnMonster{Slug: "goblin", Count: 1}))
	refuse("No such monster: dragon.", spawn(0, 0, live.SpawnMonster{Slug: "dragon", Count: 1}))
	refuse("There is no room for that many creatures there.", spawn(0, 0, live.SpawnMonster{Slug: "goblin", Count: 30}))
	w.hub.Submit(tb.player, spawn(0, 0, live.SpawnMonster{Slug: "goblin", Count: 1}))
	if u := next(t, tb.player); u.Kind != live.UpdRejected {
		t.Fatalf("a player spawned an encounter: %+v", u)
	}

	tb.dmSays(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 1, R: 0}}, On: true})
	d, p := tb.dmSays(spawn(0, 0, live.SpawnMonster{Slug: "goblin", Count: 2}, live.SpawnMonster{Slug: "wolf", Count: 1}))
	spawned := d.ActionSeq
	if spawned < 1 || p.ActionSeq != 0 {
		t.Fatalf("the spawn's action = %d, the party's %d", spawned, p.ActionSeq)
	}
	seen := map[live.Hex]bool{}
	for _, label := range []string{"Goblin 1", "Goblin 2", "Wolf"} {
		tv := tokenNamed(t, d.View, label)
		at := live.Hex{Q: tv.Q, R: tv.R}
		if seen[at] || at == (live.Hex{Q: 1, R: 0}) || tv.Kind != domain.TokenEnemy || tv.HP == nil {
			t.Fatalf("%s at %+v: %+v", label, at, tv)
		}
		seen[at] = true
	}
	if g := tokenNamed(t, d.View, "Goblin 1"); g.Q != 0 || g.R != 0 {
		t.Fatalf("the first creature takes the chosen hex: %+v", g)
	}
	goblin := tokenNamed(t, d.View, "Goblin 1").ID

	crate, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Crate", TokenKind: domain.TokenObject, Q: 4})
	refuse("No such token.", live.Command{Kind: live.CmdAdjustHP, TokenID: "nope", HPDelta: 1})
	refuse("That token has no hit points.", live.Command{Kind: live.CmdAdjustHP, TokenID: tokenNamed(t, crate.View, "Crate").ID, HPDelta: 1})
	refuse("Change the hit points by at least one.", live.Command{Kind: live.CmdAdjustHP, TokenID: goblin})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: goblin, HPDelta: -5})
	if hp := tokenNamed(t, d.View, "Goblin 1").HP; *hp != 2 {
		t.Fatalf("7 - 5 = %d", *hp)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: goblin, HPDelta: 100})
	healed := d.ActionSeq
	if hp := tokenNamed(t, d.View, "Goblin 1").HP; *hp != 7 {
		t.Fatalf("healing stops at the maximum: %d", *hp)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdUndo, Seq: healed})
	if hp := tokenNamed(t, d.View, "Goblin 1").HP; *hp != 2 {
		t.Fatalf("undoing the heal: %d", *hp)
	}
	refuse("That action is already undone.", live.Command{Kind: live.CmdUndo, Seq: healed})
	refuse("No such action in this session.", live.Command{Kind: live.CmdUndo, Seq: 99999})

	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: goblin, Effect: "bless"})
	blessed := d.ActionSeq
	d, _ = tb.dmSays(live.Command{Kind: live.CmdUndo, Seq: blessed})
	if fx := tokenNamed(t, d.View, "Goblin 1").Effects; len(fx) != 0 {
		t.Fatalf("undoing an effect ends it: %+v", fx)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: goblin, Effect: "bless"})
	again := d.ActionSeq
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: tokenNamed(t, d.View, "Goblin 1").Effects[0].ID})
	refuse("That effect has already ended.", live.Command{Kind: live.CmdUndo, Seq: again})

	d, _ = tb.dmSays(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 3, R: 0}}, On: true})
	shown := d.ActionSeq
	tb.dmSays(live.Command{Kind: live.CmdUndo, Seq: shown})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 0, R: 0}}, On: false})
	tb.dmSays(live.Command{Kind: live.CmdUndo, Seq: d.ActionSeq})

	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Barrel", TokenKind: domain.TokenObject, Q: 4, R: 1})
	placed := d.ActionSeq
	d, _ = tb.dmSays(live.Command{Kind: live.CmdUndo, Seq: placed})
	for _, tv := range d.View.Tokens {
		if tv.Label == "Barrel" {
			t.Fatal("undoing a placement removes the token")
		}
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Barrel", TokenKind: domain.TokenObject, Q: 4, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: tokenNamed(t, d.View, "Barrel").ID})
	refuse("That token is already gone.", live.Command{Kind: live.CmdUndo, Seq: d.ActionSeq})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRest, Rest: "short"})
	refuse("Grimoire cannot undo that action.", live.Command{Kind: live.CmdUndo, Seq: d.ActionSeq})

	w.hub.Submit(tb.dm, live.Command{Nonce: "u", Kind: live.CmdUndo, Seq: spawned})
	for range 3 {
		d = next(t, tb.dm)
		next(t, tb.player)
	}
	if d.Nonce != "u" || d.ActionSeq < 1 || len(d.View.Tokens) != 1 {
		t.Fatalf("undoing the spawn removes every creature: %+v", d)
	}
	refuse("That action is already undone.", live.Command{Kind: live.CmdUndo, Seq: spawned})
	d, _ = tb.dmSays(spawn(0, 0, live.SpawnMonster{Slug: "wolf", Count: 1}))
	lone := d.ActionSeq
	wolf := tokenNamed(t, d.View, "Wolf").ID
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: wolf, HPDelta: -1})
	hurt := d.ActionSeq
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: wolf})
	refuse("Those creatures are already gone.", live.Command{Kind: live.CmdUndo, Seq: lone})
	refuse("That token is already gone.", live.Command{Kind: live.CmdUndo, Seq: hurt})
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdUndo, Seq: hurt})
	if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "Only the DM") {
		t.Fatalf("a player undid an action: %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 3, R: 1}}, On: true})
	tb.dmSays(live.Command{Kind: live.CmdSetMap})
	refuse("Choose a map first.", live.Command{Kind: live.CmdUndo, Seq: d.ActionSeq})
}

type failingAction struct{ live.Store }

func (failingAction) Action(context.Context, domain.SessionID, int64) (live.ActionRecord, error) {
	return live.ActionRecord{}, errors.New("disk on fire")
}

func TestUndoNeedsTheActionLog(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Store = failingAction{Store: w.hub.Store}
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdUndo, Seq: 1})
	if u := next(t, dm); u.Kind != live.UpdRejected || u.Reason != "The action could not be read." {
		t.Fatalf("broken log = %+v", u)
	}
}
