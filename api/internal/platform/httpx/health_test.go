package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/readyz", http.StatusOK},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
		{http.MethodGet, "/nope", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			httpx.Health().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && rec.Body.String() != `{"status":"ok"}` {
				t.Fatalf("body = %q", rec.Body.String())
			}
		})
	}
}
