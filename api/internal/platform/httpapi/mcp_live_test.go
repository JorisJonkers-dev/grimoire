package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// quietHub joins without ever answering, or closes at once, or refuses.
type quietHub struct {
	closed, refuse bool
}

func (q quietHub) Join(context.Context, playdomain.SessionID, playdomain.Member, caller.Caller, live.Audience) (*live.Subscriber, error) {
	if q.refuse {
		return nil, live.ErrClosed
	}
	out := make(chan live.Update, 1)
	if q.closed {
		close(out)
	} else {
		out <- live.Update{Kind: live.UpdSnapshot}
	}
	return &live.Subscriber{Out: out}, nil
}
func (quietHub) Leave(*live.Subscriber)                {}
func (quietHub) Submit(*live.Subscriber, live.Command) {}

type liveSessions struct{ httpapi.SessionService }

func (liveSessions) Get(context.Context, caller.Caller, uuid.UUID, playdomain.SessionID) (playdomain.Session, error) {
	return playdomain.Session{Status: "live"}, nil
}

type anyMember struct{}

func (anyMember) Membership(context.Context, uuid.UUID, string) (playdomain.Member, error) {
	return playdomain.Member{DM: true}, nil
}

func TestLiveCallsGiveUpOnASilentSession(t *testing.T) {
	t.Parallel()
	one := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/sessions/0190c7a8-0000-7000-8000-000000000002"
	cmd := `{"nonce":"x","kind":"resync","q":0,"r":0,"hidden":false}`
	for _, hub := range []quietHub{{}, {closed: true}, {refuse: true}} {
		h, err := httpapi.New(httpapi.Options{
			Handler: &httpapi.Handler{
				Store: fakeStore{}, Campaigns: brokenCampaigns{}, Sessions: liveSessions{}, Hub: hub, LiveMembers: anyMember{}, LiveTimeout: 20 * time.Millisecond, Log: quiet,
			},
			RateLimit: 1000, Now: time.Now,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range []*httptest.ResponseRecorder{call(h, http.MethodPost, one+"/commands", "dm", cmd), call(h, http.MethodGet, one+"/view", "dm", "")} {
			if r.Code != http.StatusServiceUnavailable {
				t.Errorf("%+v: %d %s", hub, r.Code, r.Body.String())
			}
		}
	}
	hh := &httpapi.Handler{Log: quiet}
	ctx := context.Background()
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetSessionView(ctx, oas.GetSessionViewParams{}))
	add(hh.SendLiveCommand(ctx, &oas.LiveCommand{}, oas.SendLiveCommandParams{}))
	add(hh.GetSessionLog(ctx, oas.GetSessionLogParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
	broken := campaignServer(t, brokenCampaigns{}, httpapi.SessionService(brokenSessions{err: context.DeadlineExceeded}))
	if rec := call(broken, http.MethodGet, one+"/log", "dm", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("broken log: %d", rec.Code)
	}
}
