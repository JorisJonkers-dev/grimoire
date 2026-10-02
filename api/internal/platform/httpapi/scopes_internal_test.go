package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

// The live socket takes an Access Token only when it may play; a session passes as before.
func TestTheLiveSocketNeedsPlay(t *testing.T) {
	t.Parallel()
	reached := false
	h := playing(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	for scopes, want := range map[string]bool{"": true, "read build": false, "read play": true} {
		reached = false
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		if scopes != "" {
			req.Header.Set(httpx.ScopesHeader, scopes)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if reached != want || (!want && rec.Code != http.StatusForbidden) {
			t.Errorf("scopes %q = reached %v, %d", scopes, reached, rec.Code)
		}
	}
}
