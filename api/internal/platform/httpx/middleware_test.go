package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(r.Header.Get(httpx.IdentityHeader)))
})

func TestDevIdentityFillsMissingSubject(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpx.DevIdentity("dev", ok).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Body.String() != "dev" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestDevIdentityKeepsExistingSubject(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set(httpx.IdentityHeader, "real")
	rec := httptest.NewRecorder()
	httpx.DevIdentity("dev", ok).ServeHTTP(rec, req)
	if rec.Body.String() != "real" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func do(h http.Handler, path, subject, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.RemoteAddr = remote
	if subject != "" {
		req.Header.Set(httpx.IdentityHeader, subject)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiterCountsDownThenRejects(t *testing.T) {
	t.Parallel()
	c := &clock{t: time.Unix(1_000, 0)}
	l := &httpx.RateLimiter{Limit: 2, Window: time.Minute, Now: c.now}
	h := l.Wrap(ok)

	first := do(h, "/api", "a", "1.2.3.4:5")
	if first.Code != 200 || first.Header().Get("RateLimit-Remaining") != "1" || first.Header().Get("RateLimit-Limit") != "2" || first.Header().Get("RateLimit-Reset") != "60" {
		t.Fatalf("first: %d %v", first.Code, first.Header())
	}
	c.t = c.t.Add(10 * time.Second)
	if second := do(h, "/api", "a", "1.2.3.4:5"); second.Code != 200 || second.Header().Get("RateLimit-Remaining") != "0" || second.Header().Get("RateLimit-Reset") != "50" {
		t.Fatalf("second: %d %v", second.Code, second.Header())
	}
	third := do(h, "/api", "a", "1.2.3.4:5")
	if third.Code != http.StatusTooManyRequests || third.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("third: %d %v", third.Code, third.Header())
	}
	var p map[string]any
	if err := json.Unmarshal(third.Body.Bytes(), &p); err != nil || p["status"] != float64(429) {
		t.Fatalf("problem = %v %v", p, err)
	}
	if other := do(h, "/api", "b", "1.2.3.4:5"); other.Code != 200 {
		t.Fatalf("other subject limited: %d", other.Code)
	}
	c.t = c.t.Add(time.Minute)
	if again := do(h, "/api", "a", "1.2.3.4:5"); again.Code != 200 {
		t.Fatalf("window did not reset: %d", again.Code)
	}
}

func TestRateLimiterKeysAnonymousByAddress(t *testing.T) {
	t.Parallel()
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}
	h := l.Wrap(ok)
	if r := do(h, "/api", "", "9.9.9.9:1"); r.Code != 200 {
		t.Fatalf("first: %d", r.Code)
	}
	if r := do(h, "/api", "", "9.9.9.9:2"); r.Code != http.StatusTooManyRequests {
		t.Fatalf("same host, other port: %d", r.Code)
	}
	if r := do(h, "/api", "", "no-port"); r.Code != 200 {
		t.Fatalf("unparseable address: %d", r.Code)
	}
}

func TestRateLimiterExemptsProbes(t *testing.T) {
	t.Parallel()
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, Exempt: map[string]bool{"/healthz": true}}
	h := l.Wrap(ok)
	for range 3 {
		if r := do(h, "/healthz", "", "1.1.1.1:1"); r.Code != 200 || r.Header().Get("RateLimit-Limit") != "" {
			t.Fatalf("probe limited: %d", r.Code)
		}
	}
}

func TestRateLimiterLogsTheFirstRefusalOfEachWindow(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, Log: slog.New(slog.NewTextHandler(&out, nil))}
	h := l.Wrap(ok)
	for range 3 {
		do(h, "/api/x", "alice", "1.1.1.1:1")
	}
	if n := strings.Count(out.String(), "rate limit reached"); n != 1 || !strings.Contains(out.String(), "key=id:alice") {
		t.Fatalf("log = %q", out.String())
	}
	quiet := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}).Wrap(ok)
	do(quiet, "/api/x", "bob", "1.1.1.1:1")
	if r := do(quiet, "/api/x", "bob", "1.1.1.1:1"); r.Code != http.StatusTooManyRequests {
		t.Fatalf("without a logger = %d", r.Code)
	}
}
