package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	dmCaller     = caller.UI("dm")
	playerCaller = caller.UI("player")
	quiet        = slog.New(slog.NewTextHandler(io.Discard, nil))
)

type world struct {
	pool    *pgxpool.Pool
	hub     *live.Hub
	session domain.Session
	dm      domain.Member
	player  domain.Member
}

func setup(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	camp := campaignapp.NewService(campaignpg.New(store.Pool()))
	d, _ := camp.Create(ctx, dmCaller, campaignapp.CreateInput{Name: "Live", DisplayName: "Joris"})
	inv, _ := camp.CreateInvite(ctx, dmCaller, d.ID)
	if _, err := camp.AcceptInvite(ctx, playerCaller, inv.Token, "Tamsin"); err != nil {
		t.Fatal(err)
	}
	members := pgstore.CampaignMembers{Store: campaignpg.New(store.Pool())}
	dm, _ := members.Membership(ctx, uuid.UUID(d.ID), dmCaller.Subject)
	player, _ := members.Membership(ctx, uuid.UUID(d.ID), playerCaller.Subject)
	seed := uint64(0)
	hub := &live.Hub{
		Store: pgstore.New(store.Pool()), Members: members, Owner: pgstore.Owner{Pool: store.Pool()}, Now: time.Now, Log: quiet,
		Seed: func() uint64 { seed++; return seed }, Source: func(s uint64) dice.Source { return rng.New(s) },
	}
	t.Cleanup(hub.Shutdown)
	sessions := &app.Sessions{Repo: pgstore.New(store.Pool()), Members: members, Live: hub, Now: time.Now}
	s, err := sessions.Start(ctx, dmCaller, uuid.UUID(d.ID))
	if err != nil {
		t.Fatal(err)
	}
	return world{pool: store.Pool(), hub: hub, session: s, dm: dm, player: player}
}

// dungeon stores a dark 400 × 300 px map with 40 px hexes; its hexes include row r=0 from q=0 to q=5.
func (w world) dungeon(t *testing.T) domain.Map {
	t.Helper()
	return w.picture(t, "Crypt", domain.MapLocal)
}

// realm is a world map of the same size.
func (w world) realm(t *testing.T) domain.Map {
	t.Helper()
	return w.picture(t, "Realm", domain.MapWorld)
}

func (w world) picture(t *testing.T, name, kind string) domain.Map {
	t.Helper()
	m, err := pgstore.New(w.pool).InsertMap(context.Background(), domain.Map{
		CampaignID: w.session.CampaignID, Name: name, Kind: kind, ImageKey: "sha256/x.png", ImageType: "image/png", Width: 400, Height: 300,
		HexSize: 40, OriginX: 34.64, OriginY: 40,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func next(t *testing.T, sub *live.Subscriber) live.Update {
	t.Helper()
	select {
	case u, ok := <-sub.Out:
		if !ok {
			t.Fatal("subscriber was closed")
		}
		return u
	case <-time.After(5 * time.Second):
		t.Fatal("no update")
	}
	return live.Update{}
}

func join(t *testing.T, w world, m domain.Member, c caller.Caller, a live.Audience) *live.Subscriber {
	t.Helper()
	sub, err := w.hub.Join(context.Background(), w.session.ID, m, c, a)
	if err != nil {
		t.Fatal(err)
	}
	if u := next(t, sub); u.Kind != live.UpdSnapshot || u.Session.Audience != a || u.View == nil {
		t.Fatalf("first update = %+v", u)
	}
	return sub
}

func token(v *live.View, label string) *live.TokenView {
	for i := range v.Tokens {
		if v.Tokens[i].Label == label {
			return &v.Tokens[i]
		}
	}
	return nil
}

func has(hs []live.Hex, q, r int) bool {
	for _, h := range hs {
		if h.Q == q && h.R == r {
			return true
		}
	}
	return false
}

func payloads(t *testing.T, updates ...live.Update) string {
	t.Helper()
	raw, err := json.Marshal(updates)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestHiddenTokensNeverReachPlayersOrTheTable(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	var seen []live.Update
	w.hub.Submit(dm, live.Command{Nonce: "1", Kind: live.CmdPlace, Label: "Ambusher", TokenKind: domain.TokenEnemy, Q: 2, R: 1, Hidden: true})
	placed := next(t, dm)
	amb := token(placed.View, "Ambusher")
	if placed.Kind != live.UpdView || placed.Nonce != "1" || amb == nil || !amb.Hidden || placed.Seq != 1 || placed.View.Fog {
		t.Fatalf("dm sees = %+v", placed)
	}
	seen = append(seen, next(t, player), next(t, table))
	w.hub.Submit(dm, live.Command{Nonce: "2", Kind: live.CmdMove, TokenID: amb.ID, Q: 3, R: 1})
	next(t, dm)
	seen = append(seen, next(t, player), next(t, table))
	w.hub.Submit(player, live.Command{Nonce: "x", Kind: live.CmdResync})
	seen = append(seen, next(t, player))
	late := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(late, live.Command{Nonce: "y", Kind: live.CmdResync})
	seen = append(seen, next(t, late))
	for _, u := range seen {
		if len(u.View.Tokens) != 0 || u.Nonce != "" {
			t.Fatalf("hidden change leaked as %+v", u)
		}
	}
	if body := payloads(t, seen...); strings.Contains(body, amb.ID) || strings.Contains(body, "Ambusher") {
		t.Fatalf("hidden token in player or table payloads: %s", body)
	}
	if seen[0].Seq != 1 || seen[2].Seq != 2 {
		t.Fatalf("every audience gets every sequence: %+v", seen)
	}
}

func TestRevealHideMoveAndRemove(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: " Goblin ", TokenKind: domain.TokenEnemy, Hidden: true})
	id := token(next(t, dm).View, "Goblin").ID
	next(t, player)
	steps := []struct {
		cmd     live.Command
		visible bool
	}{
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: false}, true},
		{live.Command{Kind: live.CmdMove, TokenID: id, Q: 1, R: -1}, true},
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: true}, false},
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: false}, true},
		{live.Command{Kind: live.CmdRemove, TokenID: id}, false},
	}
	for i, s := range steps {
		w.hub.Submit(dm, s.cmd)
		d, p := next(t, dm), next(t, player)
		if (token(p.View, "Goblin") != nil) != s.visible || d.Seq != int64(i+2) || p.Seq != d.Seq {
			t.Fatalf("step %d: dm %+v player %+v", i, d, p)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Tamsin", TokenKind: domain.TokenParty, Q: 0, R: 1, DarkvisionFt: 60})
	if u := next(t, player); token(u.View, "Tamsin") == nil || token(u.View, "Tamsin").DarkvisionFt != 60 {
		t.Fatalf("visible placement = %+v", u)
	}
	next(t, dm)
	w.hub.Close(w.session.ID)
	reloaded, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM)
	if err != nil {
		t.Fatal(err)
	}
	if snap := next(t, reloaded); len(snap.View.Tokens) != 1 || snap.Seq != 7 {
		t.Fatalf("reloaded state = %+v", snap)
	}
	late := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Leave(reloaded)
	w.hub.Leave(late)
	if _, ok := <-late.Out; ok {
		t.Fatal("left subscriber still open")
	}
}

func TestCommandsAreValidated(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(player, live.Command{Nonce: "p", Kind: live.CmdPlace, Label: "Me", TokenKind: domain.TokenParty})
	if u := next(t, player); u.Kind != live.UpdRejected || u.Nonce != "p" || !strings.Contains(u.Reason, "Only the DM") {
		t.Fatalf("player command = %+v", u)
	}
	bad := map[string]live.Command{
		"label":   {Kind: live.CmdPlace, Label: " ", TokenKind: domain.TokenEnemy},
		"long":    {Kind: live.CmdPlace, Label: strings.Repeat("x", 41), TokenKind: domain.TokenEnemy},
		"kind":    {Kind: live.CmdPlace, Label: "A", TokenKind: "dragon"},
		"vision":  {Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenParty, DarkvisionFt: 400},
		"off":     {Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenEnemy, Q: 99},
		"token":   {Kind: live.CmdMove, TokenID: uuid.NewString()},
		"id":      {Kind: live.CmdMove, TokenID: "nope"},
		"unknown": {Kind: "teleport"},
		"mapid":   {Kind: live.CmdSetMap, MapID: "nope"},
		"nomap":   {Kind: live.CmdSetMap, MapID: uuid.NewString()},
		"reveal":  {Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 0, R: 0}}},
		"light":   {Kind: live.CmdPlaceLight},
	}
	for name, cmd := range bad {
		cmd.Nonce = name
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || u.Nonce != name || u.Seq != 0 {
			t.Errorf("%s = %+v", name, u)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenObject})
	id := token(next(t, dm).View, "A").ID
	next(t, player)
	w.hub.Submit(dm, live.Command{Nonce: "far", Kind: live.CmdMove, TokenID: id, Q: 50, R: 50})
	if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "off the map") {
		t.Fatalf("move off map = %+v", u)
	}
}

func TestFogOfWarOnADarkMap(t *testing.T) {
	t.Parallel()
	w := setup(t)
	m := w.dungeon(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	var partySaw []live.Update
	send := func(cmd live.Command) (live.Update, live.Update) {
		t.Helper()
		w.hub.Submit(dm, cmd)
		d, p := next(t, dm), next(t, player)
		if d.Kind != live.UpdView {
			t.Fatalf("%s rejected: %+v", cmd.Kind, d)
		}
		partySaw = append(partySaw, p, next(t, table))
		return d, p
	}
	send(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	send(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientDark})
	send(live.Command{Kind: live.CmdPlace, Label: "Orc", TokenKind: domain.TokenEnemy, Q: 4, R: 0})
	d, p := send(live.Command{Kind: live.CmdPlace, Label: "Aria", TokenKind: domain.TokenParty, Q: 0, R: 0, DarkvisionFt: 10})
	if !p.View.Fog || p.View.Map == nil || !has(p.View.Visible, 2, 0) || has(p.View.Visible, 3, 0) || token(p.View, "Orc") != nil {
		t.Fatalf("darkvision 10 ft = %+v", p.View)
	}
	if token(d.View, "Orc") == nil || d.View.Ambient != domain.AmbientDark {
		t.Fatalf("dm sees everything = %+v", d.View)
	}
	d, p = send(live.Command{Kind: live.CmdPlaceLight, Q: 4, R: 0, BrightFt: 5, DimFt: 10})
	if !has(p.View.Visible, 4, 0) || token(p.View, "Orc") == nil || len(d.View.Lights) != 1 {
		t.Fatalf("the light shows the orc: %+v", p.View)
	}
	lightID := d.View.Lights[0].ID
	_, p = send(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 2, R: 0}}, On: true})
	if has(p.View.Visible, 4, 0) || !has(p.View.Remembered, 4, 0) || token(p.View, "Orc") != nil {
		t.Fatalf("behind the wall the orc is only remembered ground: %+v", p.View)
	}
	_, p = send(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 2, R: 0}}, On: false})
	if token(p.View, "Orc") == nil {
		t.Fatalf("wall cleared: %+v", p.View)
	}
	d, _ = send(live.Command{Kind: live.CmdPlaceLight, Q: 5, R: 0, DimFt: 5})
	if len(d.View.Lights) != 2 {
		t.Fatalf("second light = %+v", d.View.Lights)
	}
	_, p = send(live.Command{Kind: live.CmdRemoveLight, LightID: lightID})
	if token(p.View, "Orc") == nil {
		t.Fatalf("the second light still shows the orc: %+v", p.View)
	}
	_, p = send(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 0, R: 3}}, On: true})
	if !has(p.View.Remembered, 0, 3) {
		t.Fatalf("painted reveal: %+v", p.View)
	}
	_, p = send(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 0, R: 3}}, On: false})
	if has(p.View.Remembered, 0, 3) {
		t.Fatalf("painted conceal: %+v", p.View)
	}
	for _, u := range partySaw {
		if u.View.Walls != nil || u.View.Lights != nil || u.View.Ambient != "" {
			t.Fatalf("DM-only layers reached the party: %+v", u.View)
		}
		for _, h := range append(append([]live.Hex{}, u.View.Visible...), u.View.Remembered...) {
			if h.Q == 1 && h.R == 2 {
				t.Fatalf("never-seen hex (1,2) was sent: %+v", u.View)
			}
		}
	}
	if body := payloads(t, partySaw[:6]...); strings.Contains(body, "Orc") {
		t.Fatalf("the orc was sent before anyone could see it: %s", body)
	}
	_, p = send(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientDim})
	if !has(p.View.Visible, 1, 2) {
		t.Fatalf("dim ambient lights everything in sight: %+v", p.View)
	}
	_, p = send(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientDark})
	if !has(p.View.Remembered, 1, 2) {
		t.Fatalf("back in the dark, what was seen is remembered: %+v", p.View)
	}
	w.hub.Close(w.session.ID)
	again := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(again, live.Command{Kind: live.CmdResync})
	if u := next(t, again); !has(u.View.Remembered, 1, 2) {
		t.Fatalf("reveals survive a restart: %+v", u.View)
	}
}

func TestMapCommandsAreValidated(t *testing.T) {
	t.Parallel()
	w := setup(t)
	m := w.dungeon(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	if u := next(t, dm); !u.View.Fog || u.View.Map.Name != "Crypt" || !strings.Contains(u.View.Map.ImageURL, "/maps/") {
		t.Fatalf("set map = %+v", u)
	}
	bad := map[string]live.Command{
		"none":    {Kind: live.CmdRevealHexes},
		"off":     {Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 90, R: 90}}},
		"light":   {Kind: live.CmdPlaceLight, Q: 90},
		"range":   {Kind: live.CmdPlaceLight, BrightFt: 700},
		"unlit":   {Kind: live.CmdRemoveLight, LightID: uuid.NewString()},
		"ambient": {Kind: live.CmdSetAmbient, Ambient: "twilight"},
	}
	for name, cmd := range bad {
		cmd.Nonce = name
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || u.Nonce != name {
			t.Errorf("%s = %+v", name, u)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdSetMap})
	if u := next(t, dm); u.View.Fog || u.View.Map != nil {
		t.Fatalf("clear map = %+v", u)
	}
}

type brokenStore struct {
	live.Store
}

func (brokenStore) Commit(context.Context, domain.Session, *domain.MapState, live.Write, domain.Member, caller.Caller, time.Time) (live.Committed, error) {
	return live.Committed{}, errors.New("disk full")
}

type failingLoad struct{ live.Store }

func (failingLoad) Load(context.Context, domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error) {
	return domain.Session{}, nil, nil, errors.New("gone")
}

type failingCombat struct{ live.Store }

func (failingCombat) LoadCombat(context.Context, domain.SessionID) (*domain.Combat, error) {
	return nil, errors.New("gone")
}

type failingObservations struct{ live.Store }

func (failingObservations) Observations(context.Context, domain.SessionID) (map[domain.TokenID]map[domain.TokenID]int, error) {
	return nil, errors.New("gone")
}

type failingEffects struct{ live.Store }

func (failingEffects) LoadEffects(context.Context, domain.SessionID) (domain.Effects, error) {
	return domain.Effects{}, errors.New("gone")
}

type failingTerrain struct{ live.Store }

func (failingTerrain) LoadTerrain(context.Context, domain.SessionID) (map[hex.Coord]domain.Surface, *domain.AreaCast, error) {
	return nil, nil, errors.New("gone")
}

type failingOwner struct{}

func (failingOwner) Acquire(context.Context, domain.SessionID) (func(), error) {
	return nil, errors.New("no lock")
}

func TestFailuresAndLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Store = brokenStore{Store: w.hub.Store}
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Nonce: "1", Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenObject})
	if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "could not be saved") {
		t.Fatalf("store failure = %+v", u)
	}
	other := &live.Hub{Store: pgstore.New(w.pool), Owner: pgstore.Owner{Pool: w.pool}, Now: time.Now, Log: quiet}
	if _, err := other.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("second owner = %v", err)
	}
	slow := join(t, w, w.player, playerCaller, live.AudienceParty)
	for range live.OutboxSize + 2 {
		w.hub.Submit(slow, live.Command{Kind: live.CmdResync})
	}
	drained := 0
	for range slow.Out {
		drained++
	}
	if drained != live.OutboxSize {
		t.Fatalf("slow subscriber got %d before being dropped", drained)
	}
	w.hub.Close(w.session.ID)
	if u := <-dm.Out; u.Kind != live.UpdEnded {
		t.Fatalf("close = %+v", u)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdResync})
	w.hub.Leave(dm)
	w.hub.Close(w.session.ID)
	w.hub.Store = failingLoad{}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("load failure ignored")
	}
	w.hub.Store = failingCombat{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("combat load failure ignored")
	}
	w.hub.Store = failingObservations{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("observation load failure ignored")
	}
	w.hub.Store = failingEffects{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("effect load failure ignored")
	}
	w.hub.Store = failingTerrain{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("terrain load failure ignored")
	}
	if _, err := (&live.Hub{Store: pgstore.New(w.pool), Owner: failingOwner{}}).Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("owner failure ignored")
	}
}

func TestEndedSessionsCannotBeJoined(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	members := pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}
	sessions := &app.Sessions{Repo: pgstore.New(w.pool), Members: members, Live: w.hub, Now: time.Now}
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	ended, err := sessions.End(ctx, dmCaller, w.session.CampaignID, w.session.ID)
	if err != nil || ended.Status != domain.SessionEnded || ended.EndedAt.IsZero() {
		t.Fatalf("end = %+v %v", ended, err)
	}
	if u := <-dm.Out; u.Kind != live.UpdEnded {
		t.Fatalf("dm told = %+v", u)
	}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); !errors.Is(err, live.ErrClosed) {
		t.Fatalf("join ended = %v", err)
	}
	w.hub.Shutdown()
}
