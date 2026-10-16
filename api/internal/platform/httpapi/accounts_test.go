package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

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
func (brokenAccounts) IsAdmin(context.Context, string) bool { return true }

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
