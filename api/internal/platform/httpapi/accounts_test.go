package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	identitypg "github.com/JorisJonkers-dev/grimoire/api/internal/identity/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	socialapp "github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	socialpg "github.com/JorisJonkers-dev/grimoire/api/internal/social/pgstore"
)

type outbox struct {
	mu   sync.Mutex
	sent []string
}

func (o *outbox) Send(_ context.Context, to, _, body string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = append(o.sent, to+"\n"+body)
	return nil
}

func (o *outbox) last() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.sent) == 0 {
		return ""
	}
	return o.sent[len(o.sent)-1]
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) pass(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// accountServers are one API that trusts forward-auth, for the bootstrap Admin, and one that runs on
// sessions alone, as the public route does.
func accountServers(t *testing.T) (http.Handler, http.Handler, *outbox, *clock) {
	t.Helper()
	return accountServersWith(t, func(*identityapp.Service) {})
}

func accountServersWith(t *testing.T, configure func(*identityapp.Service)) (http.Handler, http.Handler, *outbox, *clock) {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	mail, now := &outbox{}, &clock{now: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
	accounts := &identityapp.Service{
		Repo: identitypg.New(store.Pool()), Mailer: mail, Passwords: identityapp.Passwords{MemoryKiB: 64, Time: 1, Threads: 1}, Now: now.Now,
		Admins: map[string]bool{"root": true}, BaseURL: "https://grimoire.example/", Strong: httpx.Strong,
	}
	configure(accounts)
	social := &socialapp.Service{Repo: socialpg.New(store.Pool()), Now: now.Now}
	build := func(trust bool) http.Handler {
		h, err := httpapi.New(httpapi.Options{
			Handler: &httpapi.Handler{
				Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Accounts: accounts, Log: quiet, OIDCName: "jorisjonkers.dev",
				Friends: social, Conversations: social,
				Campaigns: campaignapp.NewService(campaignpg.New(store.Pool())), NPCs: &campaignapp.NPCs{Repo: campaignpg.New(store.Pool()), Now: time.Now},
				Characters: &campaignapp.Characters{
					Repo: campaignpg.New(store.Pool()), Compendium: &fakeCompendium{}, Combat: campaignapp.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now,
				},
			},
			RateLimit: 1000, Now: time.Now, Sessions: accounts, TrustForwardAuth: trust, Edits: campaignpg.New(store.Pool()),
		})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	return build(true), build(false), mail, now
}

// send makes a request with an optional session cookie and forward-auth subject.
func send(h http.Handler, method, path, cookie, subject, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: httpapi.SessionCookie, Value: cookie, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	if subject != "" {
		req.Header.Set("X-User-Id", subject)
	}
	req.Header.Set("User-Agent", "Firefox on a test bench")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// session reads the session token a response sets.
func session(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	set := rec.Header().Get("Set-Cookie")
	if !strings.Contains(set, "HttpOnly") || !strings.Contains(set, "Secure") || !strings.Contains(set, "SameSite=Lax") {
		t.Fatalf("cookie = %q", set)
	}
	value, _, _ := strings.Cut(strings.TrimPrefix(set, httpapi.SessionCookie+"="), ";")
	return value
}

func invite(t *testing.T, trusted http.Handler, body string) string {
	t.Helper()
	rec := send(trusted, http.MethodPost, "/api/v1/admin/account-invites", "", "root", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	token, _ := decode(t, rec)["token"].(string)
	return token
}

// An Admin's invite sets up one Account, which signs in on the spot and with its password later; the
// session cookie is the only identity the public route accepts.
func TestAccountsFromInviteToSignOut(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/account-invites", "", "dm", `{"hours":24}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a non-Admin invites: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodPost, "/api/v1/admin/account-invites", "", "root", `{"hours":0}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("zero hours: %d", rec.Code)
	}
	token := invite(t, trusted, `{"hours":24}`)
	if rec := send(public, http.MethodPost, "/api/v1/account-invites/preview", "", "", `{"token":"`+token+`"}`); rec.Code != 200 || decode(t, rec)["admin"] != false {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodPost, "/api/v1/account-invites/preview", "", "", `{"token":"nosuchtokennosuchtokennosuch"}`); rec.Code != http.StatusGone {
		t.Fatalf("unknown invite: %d", rec.Code)
	}
	setup := func(username string) string {
		return `{"token":"` + token + `","username":"` + username + `","nickname":"Aria","email":"aria@example.com","password":"correct horse battery"}`
	}
	if rec := send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", setup("a!b")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a bad Username: %d %s", rec.Code, rec.Body.String())
	}
	rec := send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", setup("Aria.Moon"))
	if rec.Code != http.StatusCreated || decode(t, rec)["username"] != "aria.moon" {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	cookie := session(t, rec)
	if rec := send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", setup("other")); rec.Code != http.StatusGone {
		t.Fatalf("an invite used twice: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", cookie, "", ""); rec.Code != 200 || decode(t, rec)["nickname"] != "Aria" {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", "", "root", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a client claiming an identity on the public route: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/campaigns", cookie, "", ""); rec.Code != 200 {
		t.Fatalf("campaigns with a session: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria.moon","password":"wrong wrong wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong password: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"nobody","password":"wrong wrong wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("an unknown Username: %d", rec.Code)
	}
	rec = send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":" Aria.Moon ","password":"correct horse battery"}`)
	if rec.Code != 200 {
		t.Fatalf("sign in: %d %s", rec.Code, rec.Body.String())
	}
	second := session(t, rec)
	rec = send(public, http.MethodPost, "/api/v1/sign-out", second, "", "")
	if rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("sign out: %d %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", second, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a revoked session: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", cookie, "", ""); rec.Code != 200 {
		t.Fatalf("the other device stays signed in: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-out", "", "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("signing out without a session: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodGet, "/api/v1/account", "", "root", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a forward-auth identity without an Account: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/account/password", "", "", `{"password":"0123456789"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a password without a session: %d", rec.Code)
	}
}

// An invite expires, a taken Username or email is refused, and an Admin invite makes an Admin who can
// invite in turn while an ordinary Account cannot.
func TestInvitesExpireAndAdminsInvite(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	late := invite(t, trusted, `{"hours":1}`)
	now.pass(2 * time.Hour)
	if rec := send(public, http.MethodPost, "/api/v1/account-invites/preview", "", "", `{"token":"`+late+`"}`); rec.Code != http.StatusGone {
		t.Fatalf("an expired invite: %d", rec.Code)
	}
	accept := func(token, username, email string) *httptest.ResponseRecorder {
		return send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", `{"token":"`+token+`","username":"`+username+`","nickname":"N","email":"`+email+`","password":"0123456789"}`)
	}
	if rec := accept(late, "late", "late@example.com"); rec.Code != http.StatusGone {
		t.Fatalf("accepting an expired invite: %d", rec.Code)
	}
	rec := accept(invite(t, trusted, `{"hours":24,"admin":true}`), "chief", "chief@example.com")
	if rec.Code != http.StatusCreated || decode(t, rec)["admin"] != true {
		t.Fatalf("an Admin invite: %d %s", rec.Code, rec.Body.String())
	}
	chief := session(t, rec)
	if rec := accept(invite(t, trusted, `{"hours":24}`), "chief", "other@example.com"); rec.Code != http.StatusConflict {
		t.Fatalf("a taken Username: %d", rec.Code)
	}
	if rec := accept(invite(t, trusted, `{"hours":24}`), "other", "CHIEF@example.com"); rec.Code != http.StatusConflict {
		t.Fatalf("a taken email: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", chief, "", `{"hours":24,"admin":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("an Admin Account without two-step invites: %d", rec.Code)
	}
	enableTwoStep(t, public, now, chief)
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", chief, "", `{"hours":24,"admin":true}`); rec.Code != http.StatusCreated {
		t.Fatalf("an Admin Account invites: %d %s", rec.Code, rec.Body.String())
	}
	rec = accept(invite(t, trusted, `{"hours":24}`), "plain", "plain@example.com")
	plain := session(t, rec)
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", plain, "", `{"hours":24}`); rec.Code != http.StatusForbidden {
		t.Fatalf("an ordinary Account invites: %d", rec.Code)
	}
}

// A forgotten password gets an emailed sign-in link that works once; the answer never says whether
// the email has an Account; the holder then sets a new password.
func TestSignInLinks(t *testing.T) {
	t.Parallel()
	trusted, public, mail, now := accountServers(t)
	token := invite(t, trusted, `{"hours":24}`)
	send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", `{"token":"`+token+`","username":"aria","nickname":"Aria","email":"aria@example.com","password":"0123456789"}`)
	if rec := send(public, http.MethodPost, "/api/v1/sign-in-links", "", "", `{"email":"nobody@example.com"}`); rec.Code != http.StatusAccepted || mail.last() != "" {
		t.Fatalf("an unknown email: %d %q", rec.Code, mail.last())
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in-links", "", "", `{"email":"ARIA@example.com"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("request link: %d", rec.Code)
	}
	letter := mail.last()
	i := strings.Index(letter, "https://grimoire.example/sign-in-link#")
	if !strings.HasPrefix(letter, "aria@example.com\n") || i < 0 {
		t.Fatalf("email = %q", letter)
	}
	link := strings.Fields(letter[i+len("https://grimoire.example/sign-in-link#"):])[0]
	rec := send(public, http.MethodPost, "/api/v1/sign-in-links/use", "", "", `{"token":"`+link+`"}`)
	if rec.Code != 200 {
		t.Fatalf("use link: %d %s", rec.Code, rec.Body.String())
	}
	cookie := session(t, rec)
	if rec := send(public, http.MethodPost, "/api/v1/sign-in-links/use", "", "", `{"token":"`+link+`"}`); rec.Code != http.StatusGone {
		t.Fatalf("a link used twice: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/account/password", cookie, "", `{"password":"short"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a short password: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, "/api/v1/account/password", cookie, "", `{"password":"a new long password"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("set password: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"a new long password"}`); rec.Code != 200 {
		t.Fatalf("the new password: %d", rec.Code)
	}
	send(public, http.MethodPost, "/api/v1/sign-in-links", "", "", `{"email":"aria@example.com"}`)
	stale := mail.last()
	j := strings.Index(stale, "/sign-in-link#")
	now.pass(31 * time.Minute)
	if rec := send(public, http.MethodPost, "/api/v1/sign-in-links/use", "", "", `{"token":"`+strings.Fields(stale[j+len("/sign-in-link#"):])[0]+`"}`); rec.Code != http.StatusGone {
		t.Fatalf("a link after 30 minutes: %d", rec.Code)
	}
}

// brokenAccounts fails every call the way a lost database would.
type brokenAccounts struct{}

var errAccounts = errors.New("database gone")

func (brokenAccounts) CreateInvite(context.Context, string, int, bool) (string, domain.Invite, error) {
	return "", domain.Invite{}, errAccounts
}

func (brokenAccounts) Invite(context.Context, string) (domain.Invite, error) {
	return domain.Invite{}, errAccounts
}

func (brokenAccounts) Accept(context.Context, string, domain.Setup, string) (domain.Account, string, error) {
	return domain.Account{}, "", errAccounts
}

func (brokenAccounts) SignIn(context.Context, string, string, string) (domain.SignedIn, error) {
	return domain.SignedIn{}, errAccounts
}
func (brokenAccounts) SignOut(context.Context, string) error     { return errAccounts }
func (brokenAccounts) RequestLink(context.Context, string) error { return errAccounts }
func (brokenAccounts) UseLink(context.Context, string, string) (domain.SignedIn, error) {
	return domain.SignedIn{}, errAccounts
}

func (brokenAccounts) Me(context.Context, string) (domain.Profile, error) {
	return domain.Profile{}, errAccounts
}
func (brokenAccounts) SetPassword(context.Context, string, string) error { return errAccounts }
func (brokenAccounts) UpdateProfile(context.Context, string, domain.ProfileChange) (domain.Profile, error) {
	return domain.Profile{}, errAccounts
}
func (brokenAccounts) OIDCEnabled() bool { return true }
func (brokenAccounts) StartOIDC(context.Context, string) (string, string, error) {
	return "", "", errAccounts
}

func (brokenAccounts) FinishOIDC(context.Context, string, string, string) (identityapp.OIDCOutcome, error) {
	return identityapp.OIDCOutcome{}, errAccounts
}

func (brokenAccounts) CreateFromOIDC(context.Context, string, string, string, string) (domain.Account, string, error) {
	return domain.Account{}, "", errAccounts
}

func (brokenAccounts) LinkFromOIDC(context.Context, string, string, string, string) (domain.Account, string, error) {
	return domain.Account{}, "", errAccounts
}
func (brokenAccounts) Unlink(context.Context, string) error { return errAccounts }
func (brokenAccounts) PassTwoStep(context.Context, string, string, string) (domain.Account, string, error) {
	return domain.Account{}, "", errAccounts
}

func (brokenAccounts) BeginTwoStep(context.Context, string) (identityapp.TwoStepSetup, error) {
	return identityapp.TwoStepSetup{}, errAccounts
}

func (brokenAccounts) ConfirmTwoStep(context.Context, string, string, string) ([]string, error) {
	return nil, errAccounts
}
func (brokenAccounts) DisableTwoStep(context.Context, string, string) error { return errAccounts }
func (brokenAccounts) ResetRecoveryCodes(context.Context, string, string) ([]string, error) {
	return nil, errAccounts
}

func (brokenAccounts) MintToken(context.Context, string, string, []string, int) (string, domain.AccessToken, error) {
	return "", domain.AccessToken{}, errAccounts
}

func (brokenAccounts) AccessTokens(context.Context, string) ([]domain.AccessToken, error) {
	return nil, errAccounts
}
func (brokenAccounts) RevokeToken(context.Context, string, uuid.UUID) error { return errAccounts }

func (brokenAccounts) AdminAccounts(context.Context, string) ([]domain.Listed, []domain.ListedInvite, error) {
	return nil, nil, errAccounts
}

func (brokenAccounts) AdminAccount(context.Context, string, domain.AccountID) (domain.Detail, error) {
	return domain.Detail{}, errAccounts
}

func (brokenAccounts) SendSignInLink(context.Context, string, domain.AccountID) error {
	return errAccounts
}

func (brokenAccounts) SetAdmin(context.Context, string, domain.AccountID, bool) error {
	return errAccounts
}

func (brokenAccounts) SetDisabled(context.Context, string, domain.AccountID, bool) error {
	return errAccounts
}

func (brokenAccounts) ResetTwoStep(context.Context, string, domain.AccountID) error {
	return errAccounts
}

func (brokenAccounts) History(context.Context, string) ([]domain.Event, error) {
	return nil, errAccounts
}

// When the Account store fails, every call answers 503 without saying why.
func TestAccountsWhenTheStoreFails(t *testing.T) {
	t.Parallel()
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Accounts: brokenAccounts{}, Log: quiet},
		RateLimit: 1000, Now: time.Now, TrustForwardAuth: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	link := `{"token":"abcdefghijklmnopqrstuvwxyz"}`
	for _, c := range []struct{ method, path, cookie, subject, body string }{
		{http.MethodPost, "/api/v1/admin/account-invites", "", "root", `{"hours":1}`},
		{http.MethodPost, "/api/v1/account-invites/preview", "", "", link},
		{http.MethodPost, "/api/v1/account-invites/accept", "", "", `{"token":"abcdefghijklmnopqrstuvwxyz","username":"aria","nickname":"A","email":"a@b.c","password":"0123456789"}`},
		{http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"x"}`},
		{http.MethodPost, "/api/v1/sign-out", "token", "", ""},
		{http.MethodPost, "/api/v1/sign-in-links", "", "", `{"email":"a@b.c"}`},
		{http.MethodPost, "/api/v1/sign-in-links/use", "", "", link},
		{http.MethodGet, "/api/v1/account", "", "someone", ""},
		{http.MethodPut, "/api/v1/account/password", "", "someone", `{"password":"0123456789"}`},
		{http.MethodPut, "/api/v1/account", "", "someone", `{"username":"aria","nickname":"A","email":"a@b.c"}`},
		{http.MethodPost, "/api/v1/oidc/sign-ins", "", "", ""},
		{http.MethodPost, "/api/v1/account/oidc-link", "", "someone", ""},
		{http.MethodDelete, "/api/v1/account/oidc-link", "", "someone", ""},
		{http.MethodPost, "/api/v1/oidc/accounts", "", "", `{"token":"abcdefghijklmnopqrstuvwxyz","username":"aria","nickname":"A"}`},
		{http.MethodPost, "/api/v1/oidc/links", "", "", `{"token":"abcdefghijklmnopqrstuvwxyz","username":"aria","password":"x"}`},
		{http.MethodPost, "/api/v1/sign-in/two-step", "", "", `{"challenge":"abcdefghijklmnopqrstuvwxyz","code":"123456"}`},
		{http.MethodPost, "/api/v1/account/two-step", "", "someone", ""},
		{http.MethodPost, "/api/v1/account/two-step/confirm", "", "someone", `{"code":"123456"}`},
		{http.MethodPost, "/api/v1/account/two-step/disable", "", "someone", `{"code":"123456"}`},
		{http.MethodPost, "/api/v1/account/two-step/recovery-codes", "", "someone", `{"code":"123456"}`},
		{http.MethodGet, "/api/v1/account/access-tokens", "", "someone", ""},
		{http.MethodPost, "/api/v1/account/access-tokens", "", "someone", `{"name":"x","scopes":["read"],"days":1}`},
		{http.MethodDelete, "/api/v1/account/access-tokens/0190c7a8-0000-7000-8000-0000000000c1", "", "someone", ""},
		{http.MethodGet, "/api/v1/admin/accounts", "", "root", ""},
		{http.MethodGet, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1", "", "root", ""},
		{http.MethodPost, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1/sign-in-link", "", "root", ""},
		{http.MethodPut, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1/admin", "", "root", `{"value":true}`},
		{http.MethodPut, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1/disabled", "", "root", `{"value":true}`},
		{http.MethodPost, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1/two-step/reset", "", "root", ""},
		{http.MethodGet, "/api/v1/account/history", "", "someone", ""},
	} {
		if rec := send(h, c.method, c.path, c.cookie, c.subject, c.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s = %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	if rec := send(h, http.MethodPost, "/api/v1/admin/account-invites", "", "", `{"hours":1}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("an invite without an identity = %d", rec.Code)
	}
}
