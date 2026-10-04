package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

// from makes a request with no identity at all, from an address.
func from(h http.Handler, addr, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.RemoteAddr = addr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The guides go out as the compendium works them out, for the ruleset asked for.
func TestTheCompendiumGuidesOverHTTP(t *testing.T) {
	t.Parallel()
	c := &fakeCompendium{}
	h := compendiumServer(t, c)
	rec := getWith(h, "/api/v1/compendium/guides", nil)
	want := `{"spellsByLevel":[{"level":0,"spells":[{"slug":"fire-bolt","name":"Fire Bolt","school":"evocation"}]},{"level":1,"spells":[]}],` +
		`"attacksByChallenge":[{"challenge":"1/4","monsters":2,"attacks":3,"toHitLow":4,"toHit":5,"toHitHigh":6,"damage":7}],` +
		`"lootTiers":[{"tier":2,"fromLevel":5,"toLevel":10,"rarities":["common","rare"]}],` +
		`"lootByRarity":[{"rarity":"rare","firstTier":2,"items":[{"slug":"amulet-of-health","name":"Amulet of Health"}]}]}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want || *c.guidesFor != "" {
		t.Fatalf("the guides: %d %s for %q", rec.Code, rec.Body.String(), *c.guidesFor)
	}
	if rec := getWith(h, "/api/v1/compendium/guides?ruleset=srd-2014", nil); rec.Code != http.StatusOK || *c.guidesFor != "srd-2014" {
		t.Fatalf("one ruleset: %d, asked for %q", rec.Code, *c.guidesFor)
	}
	if rec := getWith(h, "/api/v1/compendium/guides?ruleset=tome-of-secrets", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("a ruleset that is not the SRD's: %d %s", rec.Code, rec.Body.String())
	}
	broken := compendiumServer(t, &fakeCompendium{guidesErr: errors.New("disk")})
	if rec := getWith(broken, "/api/v1/compendium/guides", nil); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
		t.Fatalf("a fault: %d %s", rec.Code, rec.Body.String())
	}
}

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

// Anyone reading without an Account has a budget of requests by the address they come from: past it
// they are told to slow down, and somebody at another address is not.
func TestThePublicCompendiumIsRateLimited(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Log: quiet},
		RateLimit: 3, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"/api/v1/compendium/guides", "/api/v1/compendium/spells", "/api/v1/compendium/sources"}
	for i, path := range paths {
		rec := from(h, "203.0.113.7:4000", path)
		if rec.Code != http.StatusOK || rec.Header().Get("RateLimit-Limit") != "3" || rec.Header().Get("RateLimit-Remaining") != string(rune('2'-i)) {
			t.Fatalf("request %d: %d %v", i+1, rec.Code, rec.Header())
		}
	}
	// The fourth is refused, whichever port it comes from, and says when to come back.
	rec := from(h, "203.0.113.7:4001", "/api/v1/compendium/guides")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("RateLimit-Remaining") != "0" || rec.Header().Get("RateLimit-Reset") == "" || !strings.Contains(rec.Body.String(), "Slow down") {
		t.Fatalf("past the budget: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	if rec := from(h, "198.51.100.9:4000", "/api/v1/compendium/guides"); rec.Code != http.StatusOK {
		t.Fatalf("another address: %d", rec.Code)
	}
	// A minute on, the budget is whole again.
	now = now.Add(time.Minute)
	if rec := from(h, "203.0.113.7:4000", "/api/v1/compendium/guides"); rec.Code != http.StatusOK {
		t.Fatalf("a minute later: %d", rec.Code)
	}
}

// Behind a reverse proxy the server is told to trust, each anonymous reader has a budget of their own:
// one reader past theirs does not shut the compendium to the rest.
func TestThePublicCompendiumLimitsEachReaderBehindAProxy(t *testing.T) {
	t.Parallel()
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Log: quiet},
		RateLimit: 2, ProxyHops: 1, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	via := func(reader string) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/compendium/guides", nil)
		req.RemoteAddr = "10.0.0.1:443"
		req.Header.Set("X-Forwarded-For", reader)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if a, b, c := via("203.0.113.7"), via("203.0.113.7"), via("203.0.113.7"); a != http.StatusOK || b != http.StatusOK || c != http.StatusTooManyRequests {
		t.Fatalf("one reader: %d %d %d", a, b, c)
	}
	if got := via("198.51.100.9"); got != http.StatusOK {
		t.Fatalf("another reader through the same proxy: %d", got)
	}
	// Writing somebody else's address in front of one's own buys no new budget.
	if got := via("198.51.100.200, 203.0.113.7"); got != http.StatusTooManyRequests {
		t.Fatalf("a forged header: %d", got)
	}
}
