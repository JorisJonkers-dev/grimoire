package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc/oidctest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

func oidcServers(t *testing.T) (http.Handler, http.Handler, *oidctest.Issuer) {
	t.Helper()
	issuer := oidctest.New(t)
	trusted, public, _, _ := accountServersWith(t, func(s *identityapp.Service) {
		s.OIDC = oidc.New(oidc.Config{
			Issuer: issuer.URL, ClientID: oidctest.ClientID, ClientSecret: "secret", RedirectURL: "https://grimoire.example/oidc/callback",
			RolesClaim: "roles", HTTP: issuer.Client(),
		})
		s.Grant, s.AdminRole = "grimoire", "admin"
	})
	return trusted, public, issuer
}

// sendCookies makes a request with a session cookie and an OIDC sign-in cookie.
func sendCookies(h http.Handler, method, path, session, bound, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: session, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	if bound != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.OIDCCookie, Value: bound, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// login runs the provider round trip from a start endpoint: the browser goes to the provider, signs in
// as l and comes back to the callback with the cookie the start set.
func login(t *testing.T, h http.Handler, issuer *oidctest.Issuer, start, session string, l oidctest.Login) *httptest.ResponseRecorder {
	t.Helper()
	rec := sendCookies(h, http.MethodPost, start, session, "", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("start %s: %d %s", start, rec.Code, rec.Body.String())
	}
	set := rec.Header().Get("Set-Cookie")
	if !strings.Contains(set, "HttpOnly") || !strings.Contains(set, "Path=/api/v1/oidc") {
		t.Fatalf("oidc cookie = %q", set)
	}
	bound, _, _ := strings.Cut(strings.TrimPrefix(set, httpapi.OIDCCookie+"="), ";")
	to, _ := decode(t, rec)["url"].(string)
	code, state := issuer.Authorize(t, to, l)
	return sendCookies(h, http.MethodPost, "/api/v1/oidc/callback", "", bound, `{"code":"`+code+`","state":"`+state+`"}`)
}

func outcome(t *testing.T, rec *httptest.ResponseRecorder, want string) map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["status"] != want {
		t.Fatalf("outcome = %v, want %s", body, want)
	}
	return body
}

func pendingToken(body map[string]any) string {
	p, _ := body["pending"].(map[string]any)
	token, _ := p["token"].(string)
	return token
}

func oidcOf(t *testing.T, h http.Handler, session string) map[string]any {
	t.Helper()
	rec := sendCookies(h, http.MethodGet, "/api/v1/account", session, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("account: %d %s", rec.Code, rec.Body.String())
	}
	link, _ := decode(t, rec)["oidc"].(map[string]any)
	return link
}

var grimoireLogin = oidctest.Login{Subject: "estate-1", Email: "mira@example.org", Username: "mira", Name: "Mira Vale", Roles: []string{"grimoire"}}

// From the sign-in page, a login no Account has creates one keyed on its own subject; it signs in
// directly afterwards, and unlinking needs a password first and keeps the Account.
func TestOIDCCreatesAnAccountAndUnlinks(t *testing.T) {
	t.Parallel()
	trusted, public, issuer := oidcServers(t)
	if rec := send(public, http.MethodGet, "/api/v1/sign-in-methods", "", "", ""); rec.Code != http.StatusOK || decode(t, rec)["oidc"] != "jorisjonkers.dev" {
		t.Fatalf("sign-in methods: %d", rec.Code)
	}
	choose := outcome(t, login(t, public, issuer, "/api/v1/oidc/sign-ins", "", grimoireLogin), "choose")
	if p, _ := choose["pending"].(map[string]any); p["email"] != "mira@example.org" || p["username"] != "mira" {
		t.Fatalf("pending = %v", choose)
	}
	if rec := send(public, http.MethodPost, "/api/v1/oidc/accounts", "", "", `{"token":"`+pendingToken(choose)+`","username":"MIRA!!","nickname":"Mira"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a bad Username: %d", rec.Code)
	}
	rec := send(public, http.MethodPost, "/api/v1/oidc/accounts", "", "", `{"token":"`+pendingToken(choose)+`","username":"mira","nickname":"Mira"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if again := send(public, http.MethodPost, "/api/v1/oidc/accounts", "", "", `{"token":"`+pendingToken(choose)+`","username":"mira2","nickname":"Mira"}`); again.Code != http.StatusGone {
		t.Fatalf("a used pending login: %d", again.Code)
	}
	cookie := session(t, rec)
	if body := decode(t, rec); body["hasPassword"] != false || body["email"] != "mira@example.org" {
		t.Fatalf("created = %v", body)
	}
	if link := oidcOf(t, public, cookie); link["name"] != "Mira Vale" || link["email"] != "mira@example.org" {
		t.Fatalf("link = %v", link)
	}
	if rec := send(trusted, http.MethodGet, "/api/v1/account", "", "estate-1", ""); rec.Code != http.StatusOK || decode(t, rec)["username"] != "mira" {
		t.Fatalf("the Account keys on the login's subject: %d", rec.Code)
	}
	signedIn := outcome(t, login(t, public, issuer, "/api/v1/oidc/sign-ins", "", grimoireLogin), "signed_in")
	if a, _ := signedIn["account"].(map[string]any); a["username"] != "mira" {
		t.Fatalf("signed in = %v", signedIn)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/account/oidc-link", cookie, "", ""); rec.Code != http.StatusConflict {
		t.Fatalf("unlink without a password: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/account/password", cookie, "", `{"password":"a long passphrase"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("password: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/account/oidc-link", cookie, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodDelete, "/api/v1/account/oidc-link", cookie, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unlink twice: %d", rec.Code)
	}
	if link := oidcOf(t, public, cookie); link != nil {
		t.Fatalf("still linked: %v", link)
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"mira","password":"a long passphrase"}`); rec.Code != http.StatusOK {
		t.Fatalf("the Account stays: %d", rec.Code)
	}
	outcome(t, login(t, public, issuer, "/api/v1/oidc/sign-ins", "", grimoireLogin), "choose")
}

// A waiting login links to an existing Account from the sign-in page, and an Account links a login from
// its Account page; a login already linked elsewhere is refused.
func TestOIDCLinksBothWays(t *testing.T) {
	t.Parallel()
	trusted, public, issuer := oidcServers(t)
	aria := setUp(t, trusted, public, "aria")
	choose := outcome(t, login(t, public, issuer, "/api/v1/oidc/sign-ins", "", oidctest.Login{Subject: "estate-2", Email: "aria@example.org", Username: "aria", Name: "Aria", Roles: []string{"grimoire"}}), "choose")
	if rec := send(public, http.MethodPost, "/api/v1/oidc/links", "", "", `{"token":"`+pendingToken(choose)+`","username":"aria","password":"wrong password"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong password: %d", rec.Code)
	}
	rec := send(public, http.MethodPost, "/api/v1/oidc/links", "", "", `{"token":"`+pendingToken(choose)+`","username":"aria","password":"0123456789ab"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("link from the sign-in page: %d %s", rec.Code, rec.Body.String())
	}
	if body := decode(t, rec); body["hasPassword"] != true {
		t.Fatalf("linked = %v", body)
	}
	if link := oidcOf(t, public, aria); link["email"] != "aria@example.org" {
		t.Fatalf("aria's link = %v", link)
	}
	outcome(t, login(t, public, issuer, "/api/v1/oidc/sign-ins", "", oidctest.Login{Subject: "estate-2", Email: "aria@example.org", Username: "aria", Name: "Aria", Roles: []string{"grimoire"}}), "signed_in")

	bram := setUp(t, trusted, public, "bram")
	linked := outcome(t, login(t, public, issuer, "/api/v1/account/oidc-link", bram, oidctest.Login{Subject: "estate-3", Email: "bram@example.org", Username: "bram", Name: "Bram", Roles: []string{"grimoire"}}), "linked")
	if a, _ := linked["account"].(map[string]any); a["username"] != "bram" {
		t.Fatalf("linked = %v", linked)
	}
	cara := setUp(t, trusted, public, "cara")
	if rec := login(t, public, issuer, "/api/v1/account/oidc-link", cara, oidctest.Login{Subject: "estate-3", Roles: []string{"grimoire"}}); rec.Code != http.StatusConflict {
		t.Fatalf("a login linked elsewhere: %d", rec.Code)
	}
	if rec := login(t, public, issuer, "/api/v1/account/oidc-link", bram, oidctest.Login{Subject: "estate-4", Roles: []string{"grimoire"}}); rec.Code != http.StatusConflict {
		t.Fatalf("a second login: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/account/oidc-link", "", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("linking signed out: %d", rec.Code)
	}
}

// Without the Grimoire permission the login is refused, while the Account's password still works; the
// estate admin role makes the Account an Admin.
func TestOIDCPermissionAndAdmin(t *testing.T) {
	t.Parallel()
	trusted, public, issuer := oidcServers(t)
	dana := setUp(t, trusted, public, "dana")
	l := oidctest.Login{Subject: "estate-5", Email: "dana@example.org", Username: "dana", Name: "Dana", Roles: []string{"grimoire"}}
	outcome(t, login(t, public, issuer, "/api/v1/account/oidc-link", dana, l), "linked")
	l.Roles = []string{"other"}
	if rec := login(t, public, issuer, "/api/v1/oidc/sign-ins", "", l); rec.Code != http.StatusForbidden {
		t.Fatalf("without the permission: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"dana","password":"0123456789ab"}`); rec.Code != http.StatusOK {
		t.Fatalf("the password still works: %d", rec.Code)
	}
	l.Roles = []string{"admin"}
	rec := login(t, public, issuer, "/api/v1/oidc/sign-ins", "", l)
	signedIn := outcome(t, rec, "signed_in")
	if a, _ := signedIn["account"].(map[string]any); a["admin"] != true {
		t.Fatalf("the admin role = %v", signedIn)
	}
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", session(t, rec), "", `{"hours":1}`); rec.Code != http.StatusCreated {
		t.Fatalf("an Admin signed in externally invites: %d", rec.Code)
	}
}

// A sign-in finishes only in the browser that started it, once.
func TestOIDCCallbackIsBoundToItsBrowser(t *testing.T) {
	t.Parallel()
	_, public, issuer := oidcServers(t)
	rec := send(public, http.MethodPost, "/api/v1/oidc/sign-ins", "", "", "")
	to, _ := decode(t, rec)["url"].(string)
	code, state := issuer.Authorize(t, to, grimoireLogin)
	body := `{"code":"` + code + `","state":"` + state + `"}`
	if rec := sendCookies(public, http.MethodPost, "/api/v1/oidc/callback", "", "", body); rec.Code != http.StatusGone {
		t.Fatalf("another browser: %d", rec.Code)
	}
	if rec := sendCookies(public, http.MethodPost, "/api/v1/oidc/callback", "", state, body); rec.Code != http.StatusOK {
		t.Fatalf("its browser: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendCookies(public, http.MethodPost, "/api/v1/oidc/callback", "", state, body); rec.Code != http.StatusGone {
		t.Fatalf("again: %d", rec.Code)
	}
	rec = send(public, http.MethodPost, "/api/v1/oidc/sign-ins", "", "", "")
	to, _ = decode(t, rec)["url"].(string)
	_, state = issuer.Authorize(t, to, grimoireLogin)
	if rec := sendCookies(public, http.MethodPost, "/api/v1/oidc/callback", "", state, `{"code":"forged","state":"`+state+`"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a forged code: %d", rec.Code)
	}
}

// Without an external login set up, the sign-in page offers none and starting one is not found.
func TestOIDCNotSetUp(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	if rec := send(public, http.MethodGet, "/api/v1/sign-in-methods", "", "", ""); rec.Code != http.StatusOK || decode(t, rec)["oidc"] != nil {
		t.Fatalf("sign-in methods: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/oidc/sign-ins", "", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("start: %d", rec.Code)
	}
	if rec := sendCookies(public, http.MethodPost, "/api/v1/oidc/callback", "", "abcdefghijklmnopqrstuvwxyz", `{"code":"c","state":"abcdefghijklmnopqrstuvwxyz"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("callback: %d", rec.Code)
	}
	ella := setUp(t, trusted, public, "ella")
	if rec := send(public, http.MethodPost, "/api/v1/account/oidc-link", ella, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("link: %d", rec.Code)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/account", `{"username":"ella","nickname":"Ella","email":"e@example.org"}`},
		{http.MethodDelete, "/api/v1/account/oidc-link", ""},
	} {
		if rec := send(public, c.method, c.path, "", "", c.body); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s signed out: %d", c.method, c.path, rec.Code)
		}
	}
	if rec := send(public, http.MethodPut, "/api/v1/account", ella, "", `{"username":"ella.v","nickname":"Ella V","email":"ella.v@example.org"}`); rec.Code != http.StatusOK || decode(t, rec)["username"] != "ella.v" {
		t.Fatalf("profile: %d", rec.Code)
	}
	setUp(t, trusted, public, "finn")
	if rec := send(public, http.MethodPut, "/api/v1/account", ella, "", `{"username":"finn","nickname":"Ella","email":"e@example.org"}`); rec.Code != http.StatusConflict {
		t.Fatalf("a taken Username: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/account", ella, "", `{"username":"ELLA!","nickname":"Ella","email":"e@example.org"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a bad Username: %d", rec.Code)
	}
}

// setUp makes an Account from an invite and returns its session.
func setUp(t *testing.T, trusted, public http.Handler, name string) string {
	t.Helper()
	token := invite(t, trusted, `{"hours":1}`)
	rec := send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "",
		`{"token":"`+token+`","username":"`+name+`","nickname":"`+name+`","email":"`+name+`@example.org","password":"0123456789ab"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("set up %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return session(t, rec)
}
