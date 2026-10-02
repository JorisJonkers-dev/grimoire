package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

type sessions map[string]string

func (s sessions) Resolve(_ context.Context, token string) (string, bool) {
	subject, ok := s[token]
	return subject, ok
}

func TestSessionsSetTheIdentity(t *testing.T) {
	t.Parallel()
	var seen, agent string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, agent = r.Header.Get(httpx.IdentityHeader), httpx.UserAgent(r.Context())
	})
	known := sessions{"good": "account:1"}
	for _, c := range []struct {
		name, cookie, header string
		trust                bool
		want                 string
	}{
		{"a session", "good", "", false, "account:1"},
		{"a spoofed header", "", "root", false, ""},
		{"a spoofed header with a session", "good", "root", false, "account:1"},
		{"forward-auth", "", "estate-user", true, "estate-user"},
		{"forward-auth wins over a session", "good", "estate-user", true, "estate-user"},
		{"an unknown session", "bad", "", false, ""},
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", "probe")
		if c.cookie != "" {
			req.AddCookie(&http.Cookie{Name: "s", Value: c.cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		if c.header != "" {
			req.Header.Set(httpx.IdentityHeader, c.header)
		}
		httpx.Sessions("s", known, c.trust, next).ServeHTTP(httptest.NewRecorder(), req)
		if seen != c.want || agent != "probe" {
			t.Errorf("%s = %q %q", c.name, seen, agent)
		}
	}
	if httpx.UserAgent(context.Background()) != "" {
		t.Error("no request, no agent")
	}
}
