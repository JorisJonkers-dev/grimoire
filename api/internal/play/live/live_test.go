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
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
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
	if _, err := camp.AcceptInvite(ctx, playerCaller, inv.Token, "Ireena"); err != nil {
		t.Fatal(err)
	}
	members := pgstore.CampaignMembers{Store: campaignpg.New(store.Pool())}
	dm, _ := members.Membership(ctx, uuid.UUID(d.ID), dmCaller.Subject)
	player, _ := members.Membership(ctx, uuid.UUID(d.ID), playerCaller.Subject)
	hub := &live.Hub{Store: pgstore.New(store.Pool()), Owner: pgstore.Owner{Pool: store.Pool()}, Now: time.Now, Log: quiet}
	t.Cleanup(hub.Shutdown)
	sessions := &app.Sessions{Repo: pgstore.New(store.Pool()), Members: members, Live: hub, Now: time.Now}
	s, err := sessions.Start(ctx, dmCaller, uuid.UUID(d.ID))
	if err != nil {
		t.Fatal(err)
	}
	return world{pool: store.Pool(), hub: hub, session: s, dm: dm, player: player}
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
	if u := next(t, sub); u.Kind != live.UpdSnapshot || u.Session.Audience != a {
		t.Fatalf("first update = %+v", u)
	}
	return sub
}

// payloads records everything an audience received, as the bytes the socket would carry.
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
	collect := func(n int) {
		for range n {
			seen = append(seen, next(t, player), next(t, table))
		}
	}
	w.hub.Submit(dm, live.Command{Nonce: "1", Kind: live.CmdPlace, Label: "Ambusher", TokenKind: domain.TokenEnemy, Q: 2, R: 1, Hidden: true})
	placed := next(t, dm)
	if placed.Kind != live.UpdToken || placed.Nonce != "1" || !placed.Token.Hidden || placed.Seq != 1 {
		t.Fatalf("dm sees = %+v", placed)
	}
	collect(1)
	w.hub.Submit(dm, live.Command{Nonce: "2", Kind: live.CmdMove, TokenID: placed.Token.ID, Q: 3, R: 1})
	if u := next(t, dm); u.Kind != live.UpdToken || u.Token.Q != 3 {
		t.Fatalf("dm move = %+v", u)
	}
	collect(1)
	w.hub.Submit(player, live.Command{Nonce: "x", Kind: live.CmdResync})
	seen = append(seen, next(t, player))
	late := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(late, live.Command{Nonce: "y", Kind: live.CmdResync})
	seen = append(seen, next(t, late))
	for _, u := range seen {
		if u.Kind != live.UpdTick && u.Kind != live.UpdSnapshot {
			t.Fatalf("hidden change leaked as %+v", u)
		}
	}
	body := payloads(t, seen...)
	if strings.Contains(body, placed.Token.ID) || strings.Contains(body, "Ambusher") {
		t.Fatalf("hidden token in player or table payloads: %s", body)
	}
	if seen[0].Seq != 1 || seen[2].Seq != 2 {
		t.Fatalf("ticks keep the sequence whole: %+v", seen)
	}
}

func TestRevealHideMoveAndRemove(t *testing.T) {
	t.Parallel()
	w := setup(t)
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	player := join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(dm, live.Command{Nonce: "1", Kind: live.CmdPlace, Label: " Goblin ", TokenKind: domain.TokenEnemy, Q: 0, R: 0, Hidden: true})
	id := next(t, dm).Token.ID
	next(t, player)
	steps := []struct {
		cmd    live.Command
		player string
	}{
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: false}, live.UpdToken},
		{live.Command{Kind: live.CmdMove, TokenID: id, Q: 1, R: -1}, live.UpdToken},
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: true}, live.UpdTokenRemoved},
		{live.Command{Kind: live.CmdSetHidden, TokenID: id, Hidden: false}, live.UpdToken},
		{live.Command{Kind: live.CmdRemove, TokenID: id}, live.UpdTokenRemoved},
	}
	for i, s := range steps {
		w.hub.Submit(dm, s.cmd)
		d, p := next(t, dm), next(t, player)
		if p.Kind != s.player || d.Seq != int64(i+2) || p.Seq != d.Seq {
			t.Fatalf("step %d: dm %+v player %+v", i, d, p)
		}
	}
	w.hub.Submit(player, live.Command{Kind: live.CmdResync})
	if u := next(t, player); len(u.Tokens) != 0 || u.Seq != 6 {
		t.Fatalf("after removal = %+v", u)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "Ireena", TokenKind: domain.TokenParty, Q: 0, R: 1})
	if u := next(t, player); u.Kind != live.UpdToken || u.Token.Label != "Ireena" {
		t.Fatalf("visible placement = %+v", u)
	}
	placed := next(t, dm)
	w.hub.Submit(dm, live.Command{Nonce: "t", Kind: "teleport", TokenID: placed.Token.ID})
	if u := next(t, dm); u.Kind != live.UpdRejected || u.Reason != "Unknown command." {
		t.Fatalf("unknown command = %+v", u)
	}
	w.hub.Close(w.session.ID)
	reloaded, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM)
	if err != nil {
		t.Fatal(err)
	}
	if snap := next(t, reloaded); len(snap.Tokens) != 1 || snap.Tokens[0].Label != "Ireena" || snap.Seq != 7 {
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
		"label":  {Kind: live.CmdPlace, Label: " ", TokenKind: domain.TokenEnemy},
		"long":   {Kind: live.CmdPlace, Label: strings.Repeat("x", 41), TokenKind: domain.TokenEnemy},
		"kind":   {Kind: live.CmdPlace, Label: "A", TokenKind: "dragon"},
		"off":    {Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenEnemy, Q: 99},
		"token":  {Kind: live.CmdMove, TokenID: uuid.NewString()},
		"id":     {Kind: live.CmdMove, TokenID: "nope"},
		"unkown": {Kind: "teleport"},
	}
	for name, cmd := range bad {
		cmd.Nonce = name
		w.hub.Submit(dm, cmd)
		if u := next(t, dm); u.Kind != live.UpdRejected || u.Nonce != name || u.Seq != 0 {
			t.Errorf("%s = %+v", name, u)
		}
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdPlace, Label: "A", TokenKind: domain.TokenObject})
	id := next(t, dm).Token.ID
	next(t, player)
	w.hub.Submit(dm, live.Command{Nonce: "far", Kind: live.CmdMove, TokenID: id, Q: 50, R: 50})
	if u := next(t, dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "off the map") {
		t.Fatalf("move off map = %+v", u)
	}
}

type brokenStore struct {
	live.Store
}

func (brokenStore) Apply(context.Context, domain.Session, live.Change, domain.Member, caller.Caller, time.Time) (int64, domain.Token, error) {
	return 0, domain.Token{}, errors.New("disk full")
}

type failingLoad struct{ live.Store }

func (failingLoad) Load(context.Context, domain.SessionID) (domain.Session, []domain.Token, error) {
	return domain.Session{}, nil, errors.New("gone")
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
	if _, err := (&live.Hub{Store: pgstore.New(w.pool), Owner: failingOwner{}}).Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("owner failure ignored")
	}
}

type failingOwner struct{}

func (failingOwner) Acquire(context.Context, domain.SessionID) (func(), error) {
	return nil, errors.New("no lock")
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
