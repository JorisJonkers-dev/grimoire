package oidc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc/oidctest"
)

func provider(issuer string, client *http.Client) *oidc.Provider {
	return oidc.New(oidc.Config{Issuer: issuer, ClientID: oidctest.ClientID, ClientSecret: "", RedirectURL: "https://grimoire.example/oidc/callback", RolesClaim: "roles", HTTP: client})
}

// The provider sends the browser with a nonce and a PKCE challenge, and reads back the login the
// issuer signed, roles included.
func TestProviderRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	issuer := oidctest.New(t)
	p := provider(issuer.URL, issuer.Client())
	to, err := p.AuthURL(ctx, "state-abc", "nonce-abc", "verifier-0123456789-0123456789-0123456789-01")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(to, issuer.URL+"/authorize?") || !strings.Contains(to, "nonce=nonce-abc") || !strings.Contains(to, "scope=openid+profile+email") {
		t.Fatalf("url = %s", to)
	}
	code, _ := issuer.Authorize(t, to, oidctest.Login{Subject: "u1", Email: "u@example.org", Username: "u", Name: "U", Roles: []string{"grimoire", "admin"}})
	if _, err := p.Exchange(ctx, code, "the wrong verifier-0123456789-0123456789-0123"); err == nil {
		t.Fatal("a wrong verifier exchanges")
	}
	code, _ = issuer.Authorize(t, to, oidctest.Login{Subject: "u1", Email: "u@example.org", Username: "u", Name: "U", Roles: []string{"grimoire", "admin"}})
	c, err := p.Exchange(ctx, code, "verifier-0123456789-0123456789-0123456789-01")
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != issuer.URL || c.Subject != "u1" || c.Email != "u@example.org" || c.Username != "u" || c.Name != "U" || c.Nonce != "nonce-abc" || len(c.Roles) != 2 {
		t.Fatalf("claims = %+v", c)
	}
}

// A provider that is down fails each call without stopping Grimoire; a token response without an ID
// token, or with one the issuer did not sign, vouches for nobody.
func TestProviderFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	down := oidc.New(oidc.Config{Issuer: "http://127.0.0.1:1", ClientID: "c", ClientSecret: "", RedirectURL: "", RolesClaim: "roles", HTTP: nil})
	if _, err := down.AuthURL(ctx, "s", "n", "v"); err == nil {
		t.Error("a provider that is down gives a URL")
	}
	if _, err := down.Exchange(ctx, "c", "v"); err == nil {
		t.Error("a provider that is down exchanges")
	}
	for name, idToken := range map[string]string{"no id token": "", "a forged id token": "eyJhbGciOiJSUzI1NiJ9.e30.c2ln"} { //nolint:gosec // a fake token
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/token" {
				body := `{"access_token":"at","token_type":"Bearer"`
				if idToken != "" {
					body += `,"id_token":"` + idToken + `"`
				}
				_, _ = w.Write([]byte(body + "}"))
				return
			}
			_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL + `/authorize","token_endpoint":"` + srv.URL + `/token","jwks_uri":"` + srv.URL + `/jwks"}`))
		}))
		if _, err := provider(srv.URL, srv.Client()).Exchange(ctx, "c", "v"); err == nil {
			t.Errorf("%s vouches", name)
		}
		srv.Close()
	}
}
