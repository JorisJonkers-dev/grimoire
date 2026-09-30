package live_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

func TestExplorationWalksRevealFogStepByStep(t *testing.T) {
	t.Parallel()
	w := setup(t)
	m := w.dungeon(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	var partySaw []live.Update
	send := func(cmd live.Command) live.Update {
		t.Helper()
		w.hub.Submit(dm, cmd)
		if d := next(t, dm); d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		p := next(t, player)
		partySaw = append(partySaw, p, next(t, table))
		return p
	}
	send(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	send(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientDark})
	send(live.Command{Kind: live.CmdPlace, Label: "Orc", TokenKind: domain.TokenEnemy, Q: 4, R: 0})
	send(live.Command{Kind: live.CmdPlace, Label: "Spy", TokenKind: domain.TokenEnemy, Q: 1, R: 1, Hidden: true})
	p := send(live.Command{Kind: live.CmdPlace, Label: "Aria", TokenKind: domain.TokenParty, DarkvisionFt: 10, ControllerID: w.player.ID.String()})
	aria := token(p.View, "Aria")
	if aria == nil || aria.ControllerID != w.player.ID.String() {
		t.Fatalf("controlled token = %+v", p.View)
	}

	ask := func(kind string, q, r int) live.Update {
		t.Helper()
		w.hub.Submit(player, live.Command{Nonce: kind, Kind: kind, TokenID: aria.ID, Q: q, R: r})
		return next(t, player)
	}
	if u := ask(live.CmdPlanWalk, 5, 0); u.Kind != live.UpdRejected || u.Reason != "There is no way there." {
		t.Fatalf("walking into never-seen ground = %+v", u)
	}
	if u := ask(live.CmdPlanWalk, 4, 0); u.Kind != live.UpdRejected || u.Reason != "There is no way there." {
		t.Fatalf("walking onto a hidden-from-sight creature = %+v", u)
	}
	u := ask(live.CmdPlanWalk, 2, 0)
	if u.Kind != live.UpdPath || u.Nonce != live.CmdPlanWalk || u.Path.CostFt != 10 || len(u.Path.Hexes) != 3 || u.Path.Hexes[1] != (live.Hex{Q: 1, R: 0}) || u.Seq != 5 {
		t.Fatalf("preview = %+v", u)
	}

	walked := ask(live.CmdWalk, 2, 0)
	if walked.Kind != live.UpdView || walked.Seq != 6 || walked.Nonce != live.CmdWalk || len(walked.Steps) != 1 {
		t.Fatalf("walk = %+v", walked)
	}
	step := walked.Steps[0]
	if a := token(&step, "Aria"); a == nil || a.Q != 1 || has(step.Visible, 4, 0) || token(&step, "Orc") != nil {
		t.Fatalf("halfway the orc is still in the dark: %+v", step)
	}
	if a := token(walked.View, "Aria"); a == nil || a.Q != 2 || !has(walked.View.Visible, 4, 0) || token(walked.View, "Orc") == nil {
		t.Fatalf("at the end the orc is seen: %+v", walked.View)
	}
	d := next(t, dm)
	if d.Nonce != "" || len(d.Steps) != 1 || token(&d.Steps[0], "Spy") == nil {
		t.Fatalf("dm sees the walk with everything: %+v", d)
	}
	partySaw = append(partySaw, walked, next(t, table))
	if body := payloads(t, partySaw...); strings.Contains(body, "Spy") {
		t.Fatalf("hidden creature reached the party: %s", body)
	}

	refused := map[string]live.Command{
		"not yours":   {Kind: live.CmdWalk, TokenID: token(walked.View, "Orc").ID, Q: 3, R: 0},
		"no such":     {Kind: live.CmdPlanWalk, TokenID: "nope"},
		"already":     {Kind: live.CmdWalk, TokenID: aria.ID, Q: 2, R: 0},
		"only the dm": {Kind: live.CmdMove, TokenID: aria.ID},
	}
	for want, cmd := range refused {
		w.hub.Submit(player, cmd)
		if u := next(t, player); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	spy := ""
	w.hub.Submit(dm, live.Command{Kind: live.CmdResync})
	for _, tv := range next(t, dm).View.Tokens {
		if tv.Label == "Spy" {
			spy = tv.ID
		}
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdWalk, TokenID: spy, Q: 1, R: 0})
	if u := next(t, player); u.Reason != "No such token." {
		t.Fatalf("walking a creature the party cannot see = %+v", u)
	}

	w.hub.Submit(dm, live.Command{Kind: live.CmdWalk, TokenID: spy, Q: 5, R: 0})
	if u := next(t, dm); u.Kind != live.UpdView || len(u.Steps) != 3 {
		t.Fatalf("the DM walks anything anywhere: %+v", u)
	}
	if u := next(t, player); strings.Contains(payloads(t, u), "Spy") {
		t.Fatalf("hidden walk leaked: %+v", u)
	}
	next(t, table)

	w.hub.Close(w.session.ID)
	again := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(again, live.Command{Kind: live.CmdResync})
	if a := token(next(t, again).View, "Aria"); a == nil || a.Q != 2 || a.ControllerID != w.player.ID.String() {
		t.Fatalf("walk and controller survive a restart: %+v", a)
	}
}

func TestWalkingOnTheOpenGrid(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	for name, id := range map[string]string{"bad": "nope", "stranger": uuid.NewString()} {
		w.hub.Submit(dm, live.Command{Nonce: name, Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenParty, ControllerID: id})
		if u := next(t, dm); u.Reason != "No such member." {
			t.Fatalf("%s controller = %+v", name, u)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, ControllerID: w.player.ID.String()})
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Kara", TokenKind: domain.TokenParty, Q: 1, R: 0})
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Wolf", TokenKind: domain.TokenEnemy, Q: 0, R: 1})
	var brom string
	for range 3 {
		next(t, dm)
		if b := token(next(t, player).View, "Brom"); b != nil {
			brom = b.ID
		}
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdPlanWalk, TokenID: brom, Q: 2, R: 0})
	if u := next(t, player); u.Kind != live.UpdPath || u.Path.CostFt != 10 {
		t.Fatalf("walking through an ally = %+v", u)
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdPlanWalk, TokenID: brom, Q: 1, R: 0})
	if u := next(t, player); u.Kind != live.UpdRejected {
		t.Fatalf("stopping in an ally's hex = %+v", u)
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdPlanWalk, TokenID: brom, Q: 0, R: 2})
	if u := next(t, player); u.Kind != live.UpdPath || u.Path.CostFt != 15 || has(u.Path.Hexes, 0, 1) {
		t.Fatalf("walking around an enemy = %+v", u)
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdWalk, TokenID: brom, Q: -2, R: 0})
	u := next(t, player)
	if u.Kind != live.UpdView || len(u.Steps) != 1 || u.View.Fog {
		t.Fatalf("walk = %+v", u)
	}
	if next(t, dm).Kind != live.UpdView {
		t.Fatal("dm missed the walk")
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdPlanWalk, TokenID: brom, Q: 99, R: 0})
	if u := next(t, player); u.Reason != "There is no way there." {
		t.Fatalf("off the grid = %+v", u)
	}
	if _, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM); err != nil {
		t.Fatal(err)
	}
}
