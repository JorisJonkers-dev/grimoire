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
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// handshake opens a socket and closes it again, and returns the HTTP status the server answered
// the handshake with.
func handshake(t *testing.T, srv *httptest.Server, path, subject string) (int, error) {
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
	if err == nil {
		_ = conn.CloseNow()
	}
	return status, err
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

func (b brokenSessions) Log(context.Context, caller.Caller, uuid.UUID, playdomain.SessionID, int) ([]playdomain.LoggedAction, error) {
	return nil, b.err
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
	if code, err := handshake(t, srv, one+"/live?audience=party", "u"); err == nil || code != http.StatusServiceUnavailable {
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
	if code, err := handshake(t, srv, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/sessions/0190c7a8-0000-7000-8000-000000000002/live?audience=party", ""); err == nil || code != http.StatusServiceUnavailable {
		t.Fatalf("dev identity: %v", err)
	}
}
