package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

type fakeStore struct {
	pingErr  error
	created  time.Time
	queryErr error
}

func (f fakeStore) Ping(context.Context) error { return f.pingErr }

func (f fakeStore) InstanceCreatedAt(context.Context) (time.Time, error) {
	return f.created, f.queryErr
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func newServer(t *testing.T, store httpapi.StatusSource, devSubject string) http.Handler {
	t.Helper()
	h, err := httpapi.New(httpapi.Options{
		Handler:    &httpapi.Handler{Version: "1.2.3", Store: store, Log: quiet},
		DevSubject: devSubject,
		RateLimit:  100,
		Now:        time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(h http.Handler, path, subject string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if subject != "" {
		req.Header.Set("X-User-Id", subject)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestProbes(t *testing.T) {
	t.Parallel()
	up := newServer(t, fakeStore{}, "")
	for _, p := range []string{"/healthz", "/readyz"} {
		rec := get(up, p, "")
		if rec.Code != 200 || decode(t, rec)["status"] != "ok" {
			t.Fatalf("%s: %d %s", p, rec.Code, rec.Body.String())
		}
	}
	down := newServer(t, fakeStore{pingErr: errors.New("boom")}, "")
	if rec := get(down, "/healthz", ""); rec.Code != 200 {
		t.Fatalf("liveness must not depend on the database: %d", rec.Code)
	}
	rec := get(down, "/readyz", "")
	if rec.Code != 503 || decode(t, rec)["detail"] != "Try again shortly." {
		t.Fatalf("readyz down: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStatusRequiresIdentity(t *testing.T) {
	t.Parallel()
	rec := get(newServer(t, fakeStore{}, ""), "/api/v1/status", "")
	if rec.Code != 401 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status without identity: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStatusReportsVersionAndDatabase(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 9, 29, 20, 0, 0, 0, time.UTC)
	rec := get(newServer(t, fakeStore{created: created}, ""), "/api/v1/status", "u-1")
	if rec.Code != 200 {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["service"] != "grimoire" || body["version"] != "1.2.3" || body["database"] != "up" || body["startedAt"] != "2026-09-29T20:00:00Z" {
		t.Fatalf("body = %v", body)
	}
	if rec.Header().Get("RateLimit-Remaining") != "99" {
		t.Fatalf("rate limit headers missing: %v", rec.Header())
	}
}

func TestStatusUsesDevSubjectLocally(t *testing.T) {
	t.Parallel()
	if rec := get(newServer(t, fakeStore{}, "dev"), "/api/v1/status", ""); rec.Code != 200 {
		t.Fatalf("dev subject: %d", rec.Code)
	}
}

func TestStatusWhenDatabaseFails(t *testing.T) {
	t.Parallel()
	rec := get(newServer(t, fakeStore{queryErr: errors.New("boom")}, ""), "/api/v1/status", "u-1")
	if rec.Code != 503 || decode(t, rec)["title"] != "Service unavailable" {
		t.Fatalf("status db down: %d %s", rec.Code, rec.Body.String())
	}
}
