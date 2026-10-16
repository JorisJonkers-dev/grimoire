//go:build integration

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

// The Dashboard and the search answer for whoever asks, and for nobody else: a Campaign shows to its
// Members only.
func TestTheDashboardAndTheSearchOverHTTP(t *testing.T) {
	t.Parallel()
	st := newStack(t, func(*identityapp.Service) {})
	h := st.trusted
	if rec := send(h, http.MethodGet, "/api/v1/dashboard", "", "aria", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"live":[],"needs":[]}` {
		t.Fatalf("an empty Dashboard: %d %s", rec.Code, rec.Body.String())
	}
	rec := send(h, http.MethodPost, "/api/v1/campaigns", "", "aria", `{"name":"Emberfall","displayName":"Aria"}`)
	var made struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &made); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("create a Campaign: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(h, http.MethodGet, "/api/v1/search?q=ember", "", "aria", "")
	if want := `{"hits":[{"group":"campaigns","kind":"campaign","title":"Emberfall","preview":"You are the DM of this Campaign","path":"/campaigns/` + made.ID + `"}]}`; rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("the DM finds the Campaign: %d %s", rec.Code, rec.Body.String())
	}
	// Somebody who is not a Member finds nothing of it.
	if rec := send(h, http.MethodGet, "/api/v1/search?q=ember", "", "bram", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"hits":[]}` {
		t.Fatalf("a stranger searches: %d %s", rec.Code, rec.Body.String())
	}
	for name, c := range map[string]struct {
		who, path string
		want      int
	}{
		"signed out reads the Dashboard": {"", "/api/v1/dashboard", http.StatusUnauthorized},
		"signed out searches":            {"", "/api/v1/search?q=ember", http.StatusUnauthorized},
		"a search for nothing":           {"aria", "/api/v1/search", http.StatusBadRequest},
		"a search for an empty string":   {"aria", "/api/v1/search?q=", http.StatusBadRequest},
		"a search for one letter":        {"aria", "/api/v1/search?q=e", http.StatusUnprocessableEntity},
		"a search for spaces":            {"aria", "/api/v1/search?q=%20%20%20", http.StatusUnprocessableEntity},
		"a search too long":              {"aria", "/api/v1/search?q=" + strings.Repeat("e", 81), http.StatusUnprocessableEntity},
		"a search far too long":          {"aria", "/api/v1/search?q=" + strings.Repeat("e", 201), http.StatusBadRequest},
	} {
		if rec := send(h, http.MethodGet, c.path, "", c.who, ""); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := send(h, http.MethodGet, "/api/v1/search?q=e", "", "aria", ""); !strings.Contains(rec.Body.String(), "Search for 2 to 80 characters.") {
		t.Errorf("the refusal says why: %s", rec.Body.String())
	}
}
