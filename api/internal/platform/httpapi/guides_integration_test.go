//go:build integration

package httpapi_test

import (
	"net/http"
	"testing"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

// The compendium and its guides answer anyone, with no Account and no session, where everything else
// asks them to sign in.
func TestThePublicCompendiumNeedsNoAccount(t *testing.T) {
	t.Parallel()
	st := newStack(t, func(*identityapp.Service) {})
	for _, path := range []string{
		"/api/v1/compendium/spells", "/api/v1/compendium/spells/fire-bolt", "/api/v1/compendium/entries?kind=monster", "/api/v1/compendium/entries/monster/goblin",
		"/api/v1/compendium/sources", "/api/v1/compendium/automation", "/api/v1/compendium/guides",
	} {
		// The server that trusts no forwarded identity is the one the public reaches.
		if rec := from(st.public, "203.0.113.7:4000", path); rec.Code != http.StatusOK {
			t.Errorf("%s with no Account: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/campaigns", "/api/v1/dashboard", "/api/v1/search?q=fire", "/api/v1/library/entries", "/api/v1/account"} {
		if rec := from(st.public, "203.0.113.7:4000", path); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s with no Account: %d", path, rec.Code)
		}
	}
}
