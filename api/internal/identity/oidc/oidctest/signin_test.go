package oidctest_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc/oidctest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/config"
)

const (
	callback = "http://localhost:4173/oidc/callback"
	verifier = "verifier-0123456789-0123456789-0123456789-01"
	secret   = "e2e-only-client-secret" //nolint:gosec // a fake provider's test-only value
)

// estate is the fake provider as the e2e suite runs it: a confidential client with a registered
// redirect URI, and the estate's people.
func estate(t *testing.T) *oidctest.Issuer {
	t.Helper()
	return oidctest.NewWith(t, oidctest.Options{ClientID: oidctest.ClientID, ClientSecret: secret, RedirectURL: callback, People: oidctest.People()})
}

// grimoire is Grimoire's own provider client, configured as the e2e server configures it.
func grimoire(issuer *oidctest.Issuer, clientSecret, redirect string) *oidc.Provider {
	return oidc.New(oidc.Config{
		Issuer: issuer.URL, ClientID: oidctest.ClientID, ClientSecret: clientSecret, RedirectURL: redirect,
		RolesClaim: oidctest.RolesClaim, HTTP: issuer.Client(),
	})
}

// answer is what the browser got: a status, where it is sent, and the page.
type answer struct {
	status   int
	location string
	body     string
}

// visit makes a request as a browser that follows no redirects, so a test reads where it is sent.
func visit(t *testing.T, method, to string, form url.Values) answer {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, to, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return answer{status: res.StatusCode, location: res.Header.Get("Location"), body: string(body)}
}

// signIn opens the provider's sign-in page from Grimoire's authorization URL and signs in as username.
func signIn(t *testing.T, issuer *oidctest.Issuer, p *oidc.Provider, username string) answer {
	t.Helper()
	to, err := p.AuthURL(context.Background(), "state-abc", "nonce-abc", verifier)
	if err != nil {
		t.Fatal(err)
	}
	if page := visit(t, http.MethodGet, to, nil); page.status != http.StatusOK || !strings.Contains(page.body, `name="username"`) {
		t.Fatalf("sign-in page: %d %s", page.status, page.body)
	}
	u, _ := url.Parse(to)
	return visit(t, http.MethodPost, issuer.URL+"/authorize", url.Values{"request": {u.RawQuery}, "username": {username}})
}

// codeFrom reads the code from where the provider sends the browser back, checking it is Grimoire's
// callback with the state Grimoire sent.
func codeFrom(t *testing.T, a answer) string {
	t.Helper()
	if a.status != http.StatusSeeOther {
		t.Fatalf("sign-in: %d %s", a.status, a.body)
	}
	back, err := url.Parse(a.location)
	if err != nil || !strings.HasPrefix(back.String(), callback+"?") || back.Query().Get("state") != "state-abc" {
		t.Fatalf("sent back to %q", a.location)
	}
	return back.Query().Get("code")
}

// Someone signing in on the provider's page with a tag gets a fresh identity of their kind, which
// Grimoire's own client exchanges and reads with the estate's roles.
func TestSigningInOnThePageVouchesForAFreshIdentityOfThatKind(t *testing.T) {
	t.Parallel()
	issuer := estate(t)
	p := grimoire(issuer, secret, callback)
	c, err := p.Exchange(context.Background(), codeFrom(t, signIn(t, issuer, p, "player+run1")), verifier)
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != issuer.URL || c.Subject != "fake-player+run1" || c.Username != "player-run1" || c.Email != "player+run1@example.org" ||
		c.Name == "" || c.Nonce != "nonce-abc" || !slices.Equal(c.Roles, []string{oidctest.GrantRole}) {
		t.Fatalf("claims = %+v", c)
	}
}

// The page knows someone with the Grimoire grant, someone without it, and an estate admin.
func TestThePeopleHoldTheGrantTheAdminRoleOrNeither(t *testing.T) {
	t.Parallel()
	want := map[string][]string{"player": {oidctest.GrantRole}, "outsider": {"SERVICE_OTHER"}, "admin": {oidctest.GrantRole, oidctest.AdminRole}}
	for username, roles := range want {
		t.Run(username, func(t *testing.T) {
			t.Parallel()
			issuer := estate(t)
			p := grimoire(issuer, secret, callback)
			c, err := p.Exchange(context.Background(), codeFrom(t, signIn(t, issuer, p, username)), verifier)
			if err != nil {
				t.Fatal(err)
			}
			if c.Subject != "fake-"+username || !slices.Equal(c.Roles, roles) {
				t.Fatalf("claims = %+v", c)
			}
		})
	}
}

// Someone the page does not know is told so and never sent back to Grimoire.
func TestThePageRefusesSomeoneItDoesNotKnow(t *testing.T) {
	t.Parallel()
	issuer := estate(t)
	got := signIn(t, issuer, grimoire(issuer, secret, callback), "stranger")
	if got.status != http.StatusUnauthorized || got.location != "" || !strings.Contains(got.body, "No such person") {
		t.Fatalf("refusal: %d %q %s", got.status, got.location, got.body)
	}
}

// Like a real provider, the page sends a browser back only to the client's registered redirect URI.
func TestThePageServesNoRequestForAnotherRedirectURI(t *testing.T) {
	t.Parallel()
	issuer := estate(t)
	to, err := grimoire(issuer, secret, "https://elsewhere.example/oidc/callback").AuthURL(context.Background(), "s", "n", verifier)
	if err != nil {
		t.Fatal(err)
	}
	if got := visit(t, http.MethodGet, to, nil); got.status != http.StatusBadRequest {
		t.Fatalf("another redirect URI: %d", got.status)
	}
}

// A client that does not hold the provider's secret exchanges nothing.
func TestAClientWithTheWrongSecretExchangesNothing(t *testing.T) {
	t.Parallel()
	issuer := estate(t)
	p := grimoire(issuer, "not-the-secret", callback)
	if _, err := p.Exchange(context.Background(), codeFrom(t, signIn(t, issuer, p, "player")), verifier); err == nil {
		t.Fatal("a wrong client secret exchanges")
	}
}

// The fake speaks the roles claim and role names Grimoire reads when nothing overrides them, so the e2e
// server runs with jorisjonkers.dev's defaults.
func TestTheFakeUsesTheRolesGrimoireReadsByDefault(t *testing.T) {
	t.Parallel()
	c, err := config.Load(func(k string) string {
		return map[string]string{
			"GRIMOIRE_DATABASE_URL": "postgres://x", "GRIMOIRE_BASE_URL": "http://localhost:4173",
			"GRIMOIRE_OIDC_ISSUER": "http://127.0.0.1:18766", "GRIMOIRE_OIDC_CLIENT_ID": oidctest.ClientID,
		}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.OIDC.RolesClaim != oidctest.RolesClaim || c.OIDC.GrantRole != oidctest.GrantRole || c.OIDC.AdminRole != oidctest.AdminRole {
		t.Fatalf("Grimoire reads %+v", *c.OIDC)
	}
}
