package live_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

type failingTable struct{ live.Store }

func (failingTable) LoadTable(context.Context, domain.SessionID) (domain.TableDisplay, error) {
	return domain.TableDisplay{}, errors.New("gone")
}

type failingWorld struct{ live.Store }

func (failingWorld) LoadMap(context.Context, uuid.UUID, domain.MapID) (*domain.MapState, error) {
	return nil, errors.New("gone")
}

func TestTheDMSteersTheTableDisplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	m := w.realm(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	w.hub.Submit(table, live.Command{Kind: live.CmdResync})
	if tv := next(t, table).View.Table; tv == nil || tv.Camera != domain.CameraFollowTurn || tv.ZoomPct != 100 || tv.Scene != domain.SceneLocal || tv.Blackout {
		t.Fatalf("the default table = %+v", tv)
	}
	say := func(cmd live.Command) (live.Update, live.Update) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		d := next(t, dm)
		if d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		next(t, player)
		return d, next(t, table)
	}
	say(live.Command{Kind: live.CmdPlace, Label: "Goblin", TokenKind: domain.TokenEnemy})
	_, tb := say(live.Command{Kind: live.CmdTableCamera, Camera: domain.CameraFree, Q: 2, R: -1, ZoomPct: 150})
	if tv := tb.View.Table; tv.Camera != domain.CameraFree || tv.Q != 2 || tv.R != -1 || tv.ZoomPct != 150 {
		t.Fatalf("free camera = %+v", tv)
	}
	_, tb = say(live.Command{Kind: live.CmdTableScene, Scene: domain.SceneTitle, Title: " Chapter One ", Body: "The mists close in."})
	if tv := tb.View.Table; tv.Scene != domain.SceneTitle || tv.Title != "Chapter One" || tv.Body != "The mists close in." || tv.Camera != domain.CameraFree {
		t.Fatalf("title card = %+v", tv)
	}
	_, tb = say(live.Command{Kind: live.CmdTableScene, Scene: domain.SceneWorld, MapID: uuid.UUID(m.ID).String()})
	if wm := tb.View.Table.WorldMap; wm == nil || wm.Name != "Realm" || !strings.HasSuffix(wm.ImageURL, "/image") || tb.View.Table.Title != "" || wm.GridKind != domain.GridSquares || wm.GridStrength != 35 {
		t.Fatalf("world scene = %+v", tb.View.Table)
	}
	d, tb := say(live.Command{Kind: live.CmdTableBlackout, On: true})
	if !tb.View.Table.Blackout || len(tb.View.Tokens) != 0 || tb.View.Table.WorldMap == nil || len(d.View.Tokens) != 1 {
		t.Fatalf("blackout empties the table and nothing else = table %+v dm %+v", tb.View, d.View.Tokens)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPing, Q: 1, R: 0})
	for _, s := range []*live.Subscriber{dm, player, table} {
		if u := next(t, s); u.Kind != live.UpdPing || *u.Ping != (live.Hex{Q: 1, R: 0}) {
			t.Fatalf("ping = %+v", u)
		}
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdPing})
	if u := next(t, player); u.Reason != "Only the DM can change the table." {
		t.Fatalf("a player ping = %+v", u)
	}
	for want, cmd := range map[string]live.Command{
		"the camera follows": {Kind: live.CmdTableCamera, Camera: "orbit", ZoomPct: 100},
		"zoom runs":          {Kind: live.CmdTableCamera, Camera: domain.CameraFree, ZoomPct: 40},
		"scenes are":         {Kind: live.CmdTableScene, Scene: "credits"},
		"titles run":         {Kind: live.CmdTableScene, Scene: domain.SceneTitle, Title: strings.Repeat("x", 81)},
		"choose a world map": {Kind: live.CmdTableScene, Scene: domain.SceneWorld, MapID: uuid.NewString()},
		"a world map for":    {Kind: live.CmdTableScene, Scene: domain.SceneWorld, MapID: uuid.UUID(w.dungeon(t).ID).String()},
	} {
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	w.hub.Close(w.session.ID)
	again := join(t, w, w.player, playerCaller, live.AudienceTable)
	w.hub.Submit(again, live.Command{Kind: live.CmdResync})
	if u := next(t, again); !u.View.Table.Blackout || u.View.Table.WorldMap == nil || u.View.Table.ZoomPct != 150 {
		t.Fatalf("the table survives a restart = %+v", u.View.Table)
	}
	dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdTableBlackout})
	next(t, dm)
	if u := next(t, again); u.View.Table.Blackout || len(u.View.Tokens) != 1 {
		t.Fatalf("lights back on = %+v", u.View)
	}
	w.hub.Close(w.session.ID)
	for name, store := range map[string]live.Store{"table": failingTable{Store: pgstore.New(w.pool)}, "world": failingWorld{Store: pgstore.New(w.pool)}} {
		w.hub.Store = store
		if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
			t.Errorf("%s load failure ignored", name)
		}
	}
}
