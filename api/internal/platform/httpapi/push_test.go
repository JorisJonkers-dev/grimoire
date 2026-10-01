package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/push"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

type devices struct{ id uuid.UUID }

func (devices) Key() string { return "BKey" }

func (d devices) Subscribe(_ context.Context, subject, endpoint, _, _ string) (uuid.UUID, error) {
	if endpoint == "https://push.example/refused" || subject != "player" {
		return uuid.UUID{}, push.ErrBadEndpoint
	}
	return d.id, nil
}

func (d devices) Unsubscribe(_ context.Context, _ string, id uuid.UUID) error {
	if id != d.id {
		return apperr.ErrNotFound
	}
	return nil
}

func TestDevicesSubscribeToNotifications(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	server := func(p httpapi.PushService) http.Handler {
		h, err := httpapi.New(httpapi.Options{Handler: &httpapi.Handler{Store: fakeStore{}, Campaigns: brokenCampaigns{}, Push: p, Log: quiet}, RateLimit: 1000, Now: time.Now})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	sub := func(endpoint string) string {
		return `{"endpoint":"` + endpoint + `","keys":{"p256dh":"BAbc","auth":"xyz"}}`
	}
	on, off := server(devices{id: id}), server(nil)
	if rec := call(on, http.MethodGet, "/api/v1/push/key", "player", ""); rec.Code != 200 || decode(t, rec)["publicKey"] != "BKey" {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(on, http.MethodPost, "/api/v1/push/subscriptions", "player", sub("https://push.example/a")); rec.Code != http.StatusCreated || decode(t, rec)["id"] != id.String() {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(on, http.MethodPost, "/api/v1/push/subscriptions", "player", sub("https://push.example/refused")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("refused: %d", rec.Code)
	}
	if rec := call(on, http.MethodDelete, "/api/v1/push/subscriptions/"+id.String(), "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unsubscribe: %d", rec.Code)
	}
	if rec := call(on, http.MethodDelete, "/api/v1/push/subscriptions/"+uuid.NewString(), "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown device: %d", rec.Code)
	}
	for _, rec := range []int{
		call(off, http.MethodGet, "/api/v1/push/key", "player", "").Code,
		call(off, http.MethodPost, "/api/v1/push/subscriptions", "player", sub("https://push.example/a")).Code,
		call(off, http.MethodDelete, "/api/v1/push/subscriptions/"+id.String(), "player", "").Code,
	} {
		if rec != http.StatusNotFound {
			t.Fatalf("push off: %d", rec)
		}
	}
	hh := &httpapi.Handler{Log: quiet}
	ctx := context.Background()
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetPushKey(ctx))
	add(hh.CreatePushSubscription(ctx, &oas.PushSubscriptionInput{}))
	add(hh.DeletePushSubscription(ctx, oas.DeletePushSubscriptionParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
