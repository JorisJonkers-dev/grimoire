package httpapi_test

import (
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
func TestAdminControls(t *testing.T) {
	t.Parallel()
	trusted, public, mail, now := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	id := accountID(t, public, aria)
	base := "/api/v1/admin/accounts/" + id
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, base + "/sign-in-link", ""},
		{http.MethodPut, base + "/admin", `{"value":true}`},
		{http.MethodPut, base + "/disabled", `{"value":true}`},
		{http.MethodPost, base + "/two-step/reset", ""},
	} {
		if rec := send(public, c.method, c.path, aria, "", c.body); rec.Code != http.StatusForbidden {
			t.Errorf("a non-Admin %s %s: %d", c.method, c.path, rec.Code)
		}
	}

	if rec := send(trusted, http.MethodPost, base+"/sign-in-link", "", "root", ""); rec.Code != http.StatusAccepted || !strings.Contains(mail.last(), "aria@example.org") {
		t.Fatalf("sign-in link: %d %q", rec.Code, mail.last())
	}
	enableTwoStep(t, public, now, aria)
	if rec := send(trusted, http.MethodPost, base+"/two-step/reset", "", "root", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("reset two-step: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"0123456789ab"}`); rec.Code != http.StatusOK {
		t.Fatalf("after the reset, a password alone: %d", rec.Code)
	}

	if rec := send(trusted, http.MethodPut, base+"/admin", "", "root", `{"value":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("make Admin: %d", rec.Code)
	}
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", aria, "", "")); body["admin"] != true {
		t.Fatalf("aria = %v", body)
	}
	secret, _ := enableTwoStep(t, public, now, aria)
	if rec := send(public, http.MethodPut, base+"/admin", aria, "", `{"value":false}`); rec.Code != http.StatusConflict {
		t.Fatalf("an Admin removes their own role: %d", rec.Code)
	}
	if rec := send(public, http.MethodPut, base+"/disabled", aria, "", `{"value":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("an Admin disables themselves: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodPut, base+"/admin", "", "root", `{"value":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke Admin: %d", rec.Code)
	}

	token, _ := mint(t, public, aria, `{"name":"Agent","scopes":["read"],"days":7}`)
	if rec := send(trusted, http.MethodPut, base+"/disabled", "", "root", `{"value":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("disable: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", aria, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a disabled Account's session: %d", rec.Code)
	}
	if rec := bearer(public, http.MethodGet, "/api/v1/campaigns", token, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a disabled Account's token: %d", rec.Code)
	}
	if rec := send(trusted, http.MethodPost, base+"/sign-in-link", "", "root", ""); rec.Code != http.StatusConflict {
		t.Fatalf("a link for a disabled Account: %d", rec.Code)
	}
	rec := send(trusted, http.MethodGet, base, "", "root", "")
	if d := decode(t, rec); d["status"] != "disabled" || d["sessions"] != float64(0) || d["tokens"] != float64(0) {
		t.Fatalf("disabled detail = %v", d)
	}
	if rec := send(trusted, http.MethodPut, base+"/disabled", "", "root", `{"value":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("enable: %d", rec.Code)
	}
	if rec := send(public, http.MethodGet, "/api/v1/account", aria, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("enabling does not revive old sessions: %d", rec.Code)
	}
	now.pass(30 * time.Second)
	again := session(t, secondStep(public, challenge(t, public, "aria"), identityapp.TOTP(secret, now.Now())))
	got := strings.Join(actions(t, send(trusted, http.MethodGet, base, "", "root", "")), " ")
	for _, want := range []string{"aria:created", "root:sign_in_link_sent", "aria:two_step_on", "root:two_step_reset", "root:admin_granted", "root:admin_revoked", "root:disabled", "root:enabled"} {
		if !strings.Contains(got, want) {
			t.Errorf("history lacks %s: %s", want, got)
		}
	}
	if own := strings.Join(actions(t, send(public, http.MethodGet, "/api/v1/account/history", again, "", "")), " "); own != got {
		t.Errorf("own history = %s, want %s", own, got)
	}
}
