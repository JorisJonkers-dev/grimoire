//go:build integration

package httpapi_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

func accountID(t *testing.T, h http.Handler, session string) string {
	t.Helper()
	id, _ := decode(t, send(h, http.MethodGet, "/api/v1/account", session, "", ""))["id"].(string)
	return id
}

func actions(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var out []string
	body := decode(t, rec)
	items, _ := body["history"].([]any)
	if items == nil {
		items, _ = body["items"].([]any)
	}
	for _, e := range items {
		ev, _ := e.(map[string]any)
		out = append(out, ev["actor"].(string)+":"+ev["action"].(string))
	}
	return out
}

// Admins see every Account and unused Invite in one list, and each Account on its own page.
func TestAdminListsEveryone(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	invite(t, trusted, `{"hours":1}`)
	invite(t, trusted, `{"hours":48,"admin":true}`)
	now.pass(2 * time.Hour)
	if rec := send(public, http.MethodGet, "/api/v1/admin/accounts", aria, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a non-Admin lists: %d", rec.Code)
	}
	rec := send(trusted, http.MethodGet, "/api/v1/admin/accounts", "", "root", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"username":"aria"`, `"status":"active"`, `"status":"invited"`, `"status":"expired"`, `"lastSeenAt"`} {
		if !strings.Contains(body, want) {
			t.Errorf("list lacks %s: %s", want, body)
		}
	}
	id := accountID(t, public, aria)
	rec = send(trusted, http.MethodGet, "/api/v1/admin/accounts/"+id, "", "root", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	detail := decode(t, rec)
	if detail["sessions"] != float64(1) || detail["status"] != "active" || len(detail["campaigns"].([]any)) != 0 {
		t.Fatalf("detail = %v", detail)
	}
	if got := actions(t, rec); len(got) != 1 || got[0] != "aria:created" {
		t.Fatalf("history = %v", got)
	}
	if rec := send(trusted, http.MethodGet, "/api/v1/admin/accounts/0190c7a8-0000-7000-8000-0000000000c1", "", "root", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a missing Account: %d", rec.Code)
	}
}

// Each Admin control works, lands in the Account's history, and is closed to non-Admins and to the
// Admin's own Account where it would lock them out.
// adminDesk is Aria's Account, seen from the Admin controls that root uses.
type adminDesk struct {
	trusted, public http.Handler
	mail            *outbox
	now             *clock
	aria, base      string
}

func newAdminDesk(t *testing.T) *adminDesk {
	t.Helper()
	trusted, public, mail, now := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	return &adminDesk{trusted: trusted, public: public, mail: mail, now: now, aria: aria, base: "/api/v1/admin/accounts/" + accountID(t, public, aria)}
}

func (a *adminDesk) root(method, path, body string) *httptest.ResponseRecorder {
	return send(a.trusted, method, a.base+path, "", "root", body)
}

func (a *adminDesk) disabled(t *testing.T, value bool) {
	t.Helper()
	if rec := a.root(http.MethodPut, "/disabled", fmt.Sprintf(`{"value":%v}`, value)); rec.Code != http.StatusNoContent {
		t.Fatalf("disabled = %v: %d", value, rec.Code)
	}
}

func TestOnlyAnAdminUsesTheAdminControls(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/sign-in-link", ""},
		{http.MethodPut, "/admin", `{"value":true}`},
		{http.MethodPut, "/disabled", `{"value":true}`},
		{http.MethodPost, "/two-step/reset", ""},
	} {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			if rec := send(a.public, c.method, a.base+c.path, a.aria, "", c.body); rec.Code != http.StatusForbidden {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestAnAdminSendsASignInLink(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	if rec := a.root(http.MethodPost, "/sign-in-link", ""); rec.Code != http.StatusAccepted || !strings.Contains(a.mail.last(), "aria@example.org") {
		t.Fatalf("%d %q", rec.Code, a.mail.last())
	}
}

func TestAfterAnAdminResetsTwoStepAPasswordAloneSignsIn(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	enableTwoStep(t, a.public, a.now, a.aria)
	if rec := a.root(http.MethodPost, "/two-step/reset", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d", rec.Code)
	}
	if rec := send(a.public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"0123456789ab"}`); rec.Code != http.StatusOK {
		t.Fatalf("sign in: %d", rec.Code)
	}
}

func TestAnAccountMadeAdminSeesItIsOne(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	if rec := a.root(http.MethodPut, "/admin", `{"value":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("make Admin: %d", rec.Code)
	}
	if body := decode(t, send(a.public, http.MethodGet, "/api/v1/account", a.aria, "", "")); body["admin"] != true {
		t.Fatalf("aria = %v", body)
	}
}

func TestAnAdminCannotRemoveTheirOwnRoleOrDisableThemselves(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	a.root(http.MethodPut, "/admin", `{"value":true}`)
	enableTwoStep(t, a.public, a.now, a.aria)
	for path, body := range map[string]string{"/admin": `{"value":false}`, "/disabled": `{"value":true}`} {
		t.Run(path, func(t *testing.T) {
			if rec := send(a.public, http.MethodPut, a.base+path, a.aria, "", body); rec.Code != http.StatusConflict {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestADisabledAccountsSessionsAndTokensStopWorking(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	token, _ := mint(t, a.public, a.aria, `{"name":"Agent","scopes":["read"],"days":7}`)
	a.disabled(t, true)
	if rec := send(a.public, http.MethodGet, "/api/v1/account", a.aria, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("session: %d", rec.Code)
	}
	if rec := bearer(a.public, http.MethodGet, "/api/v1/campaigns", token, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token: %d", rec.Code)
	}
}

func TestNoSignInLinkIsSentToADisabledAccount(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	a.disabled(t, true)
	if rec := a.root(http.MethodPost, "/sign-in-link", ""); rec.Code != http.StatusConflict {
		t.Fatalf("%d", rec.Code)
	}
}

func TestADisabledAccountIsShownWithNoSessionsOrTokens(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	mint(t, a.public, a.aria, `{"name":"Agent","scopes":["read"],"days":7}`)
	a.disabled(t, true)
	if d := decode(t, a.root(http.MethodGet, "", "")); d["status"] != "disabled" || d["sessions"] != float64(0) || d["tokens"] != float64(0) {
		t.Fatalf("detail = %v", d)
	}
}

func TestEnablingAnAccountDoesNotReviveItsOldSessions(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	a.disabled(t, true)
	a.disabled(t, false)
	if rec := send(a.public, http.MethodGet, "/api/v1/account", a.aria, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestTheAccountHistoryRecordsEveryAdminActionAndTheOwnerSeesTheSame(t *testing.T) {
	t.Parallel()
	a := newAdminDesk(t)
	a.root(http.MethodPost, "/sign-in-link", "")
	enableTwoStep(t, a.public, a.now, a.aria)
	a.root(http.MethodPost, "/two-step/reset", "")
	a.root(http.MethodPut, "/admin", `{"value":true}`)
	secret, _ := enableTwoStep(t, a.public, a.now, a.aria)
	a.root(http.MethodPut, "/admin", `{"value":false}`)
	a.disabled(t, true)
	a.disabled(t, false)
	a.now.pass(30 * time.Second)
	again := session(t, secondStep(a.public, challenge(t, a.public, "aria"), identityapp.TOTP(secret, a.now.Now())))
	got := strings.Join(actions(t, a.root(http.MethodGet, "", "")), " ")
	for _, want := range []string{"aria:created", "root:sign_in_link_sent", "aria:two_step_on", "root:two_step_reset", "root:admin_granted", "root:admin_revoked", "root:disabled", "root:enabled"} {
		if !strings.Contains(got, want) {
			t.Errorf("history lacks %s: %s", want, got)
		}
	}
	if own := strings.Join(actions(t, send(a.public, http.MethodGet, "/api/v1/account/history", again, "", "")), " "); own != got {
		t.Errorf("own history = %s, want %s", own, got)
	}
}
