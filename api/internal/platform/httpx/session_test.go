package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

type sessions map[string]string

func (s sessions) Resolve(_ context.Context, token string) (string, bool, bool) {
	subject, ok := s[token]
	return subject, token == "strong", ok
}

func TestSessionsSetTheIdentity(t *testing.T) {
	t.Parallel()
	var seen, agent string
	var strong bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, agent, strong = r.Header.Get(httpx.IdentityHeader), httpx.UserAgent(r.Context()), httpx.Strong(r.Context())
	})
	known := sessions{"good": "account:1", "strong": "account:2"}
	for _, c := range []struct {
		name, cookie, header string
		trust                bool
		want                 string
		strong               bool
	}{
		{"a session", "good", "", false, "account:1", false},
		{"a strong session", "strong", "", false, "account:2", true},
		{"a spoofed header", "", "root", false, "", false},
		{"a spoofed header with a session", "good", "root", false, "account:1", false},
		{"forward-auth", "", "estate-user", true, "estate-user", true},
		{"forward-auth wins over a session", "good", "estate-user", true, "estate-user", true},
		{"an unknown session", "bad", "", false, "", false},
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
		if seen != c.want || agent != "probe" || strong != c.strong {
			t.Errorf("%s = %q %q %v", c.name, seen, agent, strong)
		}
	}
	if httpx.UserAgent(context.Background()) != "" || httpx.Strong(context.Background()) {
		t.Error("no request, no agent")
	}
}
