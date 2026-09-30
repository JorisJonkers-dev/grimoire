package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type socket struct {
	t    *testing.T
	conn *websocket.Conn
	raw  []string
}

// dial opens a socket; on refusal it returns the HTTP status the server answered with.
func dial(t *testing.T, srv *httptest.Server, path, subject string) (*socket, int, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	if subject != "" {
		h.Set("X-User-Id", subject)
	}
	conn, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+path, &websocket.DialOptions{HTTPHeader: h}) //nolint:bodyclose // the library owns the handshake body
	status := 0
	if res != nil {
		status = res.StatusCode
	}
	if err != nil {
		return nil, status, err
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return &socket{t: t, conn: conn}, status, nil
}

func (s *socket) next() live.Update {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := s.conn.Read(ctx)
	if err != nil {
		s.t.Fatalf("read: %v", err)
	}
	s.raw = append(s.raw, string(data))
	var u live.Update
	if err := jsonUnmarshal(data, &u); err != nil {
		s.t.Fatal(err)
	}
	return u
}

func (s *socket) send(cmd live.Command) {
	s.t.Helper()
	if err := wsjson.Write(context.Background(), s.conn, cmd); err != nil {
		s.t.Fatal(err)
	}
}

func TestLiveSessionOverWebSockets(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	id, _ := campaignWithPlayer(t, h)
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player starts: %d", rec.Code)
	}
	rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions", "dm", "")
	sid, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || sid == "" {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	base := "/api/v1/campaigns/" + id + "/sessions/" + sid
	dm, _, err := dial(t, srv, base+"/live?audience=dm", "dm")
	if err != nil {
		t.Fatal(err)
	}
	player, _, _ := dial(t, srv, base+"/live?audience=party", "player")
	table, _, _ := dial(t, srv, base+"/live?audience=table", "player")
	for _, s := range []*socket{dm, player, table} {
		if u := s.next(); u.Kind != live.UpdSnapshot {
			t.Fatalf("snapshot = %+v", u)
		}
	}
	hiddenStaysHidden(t, dm, player, table)
	endSessionClosesSockets(t, h, srv, id, base, dm)
}

func hiddenStaysHidden(t *testing.T, dm, player, table *socket) {
	t.Helper()
	if err := player.conn.Write(context.Background(), websocket.MessageText, []byte("not json")); err != nil {
		t.Fatal(err)
	}
	dm.send(live.Command{Nonce: "1", Kind: live.CmdPlace, Label: "Lurker", TokenKind: "enemy", Q: 1, R: 1, Hidden: true})
	placed := dm.next()
	if placed.Token == nil || !placed.Token.Hidden {
		t.Fatalf("dm place = %+v", placed)
	}
	if u := player.next(); u.Kind != live.UpdTick {
		t.Fatalf("player saw %+v", u)
	}
	if u := table.next(); u.Kind != live.UpdTick {
		t.Fatalf("table saw %+v", u)
	}
	for _, s := range []*socket{player, table} {
		joined := strings.Join(s.raw, "\n")
		if strings.Contains(joined, "Lurker") || strings.Contains(joined, placed.Token.ID) {
			t.Fatalf("hidden token crossed the wire: %s", joined)
		}
	}
	dm.send(live.Command{Nonce: "2", Kind: live.CmdSetHidden, TokenID: placed.Token.ID, Hidden: false})
	dm.next()
	if u := player.next(); u.Kind != live.UpdToken || u.Token.Label != "Lurker" {
		t.Fatalf("reveal = %+v", u)
	}
}

func endSessionClosesSockets(t *testing.T, h http.Handler, srv *httptest.Server, id, base string, dm *socket) {
	t.Helper()
	for _, path := range []string{"", "/end"} {
		method := http.MethodGet
		if path != "" {
			method = http.MethodPost
		}
		if rec := call(h, method, base+path, "dm", ""); rec.Code != 200 {
			t.Fatalf("%s %s: %d", method, path, rec.Code)
		}
	}
	if u := dm.next(); u.Kind != live.UpdEnded {
		t.Fatalf("end = %+v", u)
	}
	if _, _, err := dm.conn.Read(context.Background()); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatalf("close = %v", err)
	}
	rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/sessions", "player", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"ended"`) || !strings.Contains(rec.Body.String(), `"endedAt"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	late, _, err := dial(t, srv, base+"/live?audience=party", "player")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := late.conn.Read(context.Background()); websocket.CloseStatus(err) != websocket.StatusTryAgainLater {
		t.Fatalf("ended session join = %v", err)
	}
}

func TestLiveSocketRefusals(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	id, _ := campaignWithPlayer(t, h)
	sid, _ := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions", "dm", ""))["id"].(string)
	base := "/api/v1/campaigns/" + id + "/sessions/" + sid + "/live"
	cases := []struct {
		path, subject string
		code          int
	}{
		{base + "?audience=party", "", http.StatusUnauthorized},
		{base + "?audience=party", "stranger", http.StatusNotFound},
		{base + "?audience=dm", "player", http.StatusForbidden},
		{base + "?audience=everyone", "player", http.StatusBadRequest},
		{"/api/v1/campaigns/" + id + "/sessions/nope/live?audience=party", "player", http.StatusNotFound},
		{"/api/v1/campaigns/" + id + "/sessions/" + uuid.NewString() + "/live?audience=party", "player", http.StatusNotFound},
	}
	if rec := call(h, http.MethodGet, base+"?audience=party", "player", ""); rec.Code != http.StatusUpgradeRequired {
		t.Errorf("plain GET: %d", rec.Code)
	}
	for _, c := range cases {
		if _, code, err := dial(t, srv, c.path, c.subject); err == nil || code != c.code {
			t.Errorf("%s as %q: %d %v, want %d", c.path, c.subject, code, err, c.code)
		}
	}
}

type brokenMembers struct{}

func (brokenMembers) Membership(context.Context, uuid.UUID, string) (playdomain.Member, error) {
	return playdomain.Member{}, errors.New("db down")
}

type brokenSessions struct{ err error }

func (b brokenSessions) Start(context.Context, caller.Caller, uuid.UUID) (playdomain.Session, error) {
	return playdomain.Session{}, b.err
}

func (b brokenSessions) Get(context.Context, caller.Caller, uuid.UUID, playdomain.SessionID) (playdomain.Session, error) {
	return playdomain.Session{}, b.err
}

func (b brokenSessions) List(context.Context, caller.Caller, uuid.UUID) ([]playdomain.Session, error) {
	return nil, b.err
}

func (b brokenSessions) End(context.Context, caller.Caller, uuid.UUID, playdomain.SessionID) (playdomain.Session, error) {
	return playdomain.Session{}, b.err
}

type idleHub struct{}

func (idleHub) Join(context.Context, playdomain.SessionID, playdomain.Member, caller.Caller, live.Audience) (*live.Subscriber, error) {
	return nil, errors.New("unused")
}
func (idleHub) Leave(*live.Subscriber)                {}
func (idleHub) Submit(*live.Subscriber, live.Command) {}

func TestSessionErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.SessionService(brokenSessions{err: errors.New("disk")}),
		httpapi.LiveHub(idleHub{}), httpapi.LiveMembers(brokenMembers{}))
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/sessions"
	one := c + "/0190c7a8-0000-7000-8000-000000000002"
	for _, o := range []struct{ method, path string }{{http.MethodGet, c}, {http.MethodPost, c}, {http.MethodGet, one}, {http.MethodPost, one + "/end"}} {
		if rec := call(h, o.method, o.path, "u", ""); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	if _, code, err := dial(t, srv, one+"/live?audience=party", "u"); err == nil || code != http.StatusServiceUnavailable {
		t.Fatalf("members down: %v", err)
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListSessions(ctx, oas.ListSessionsParams{}))
	add(hh.StartSession(ctx, oas.StartSessionParams{}))
	add(hh.GetSession(ctx, oas.GetSessionParams{}))
	add(hh.EndSession(ctx, oas.EndSessionParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}

func TestDevIdentityReachesTheSocket(t *testing.T) {
	t.Parallel()
	h, err := httpapi.New(httpapi.Options{
		Handler: &httpapi.Handler{
			Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Campaigns: brokenCampaigns{},
			Sessions: brokenSessions{err: errors.New("x")}, Hub: idleHub{}, LiveMembers: brokenMembers{}, Log: quiet,
		},
		DevSubject: "dev", RateLimit: 100, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	if _, code, err := dial(t, srv, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/sessions/0190c7a8-0000-7000-8000-000000000002/live?audience=party", ""); err == nil || code != http.StatusServiceUnavailable {
		t.Fatalf("dev identity: %v", err)
	}
}
