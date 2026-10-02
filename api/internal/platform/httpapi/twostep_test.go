package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

// enableTwoStep turns two-step on for a session's Account and returns the secret and recovery codes.
func enableTwoStep(t *testing.T, public http.Handler, now *clock, session string) (string, []string) {
	t.Helper()
	rec := send(public, http.MethodPost, "/api/v1/account/two-step", session, "", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("begin: %d %s", rec.Code, rec.Body.String())
	}
	setup := decode(t, rec)
	secret, _ := setup["secret"].(string)
	if uri, _ := setup["uri"].(string); !strings.HasPrefix(uri, "otpauth://totp/Grimoire:") || !strings.Contains(uri, "secret="+secret) || !strings.Contains(uri, "issuer=Grimoire") {
		t.Fatalf("uri = %v", setup)
	}
	rec = send(public, http.MethodPost, "/api/v1/account/two-step/confirm", session, "", `{"code":"`+identityapp.TOTP(secret, now.Now())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
	var codes []string
	for _, c := range decode(t, rec)["codes"].([]any) {
		codes = append(codes, c.(string))
	}
	return secret, codes
}

// challenge signs in with a password and returns the challenge the second step answers.
func challenge(t *testing.T, public http.Handler, username string) string {
	t.Helper()
	rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"`+username+`","password":"0123456789ab"}`)
	if rec.Code != http.StatusAccepted || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("sign in with two-step: %d %q %s", rec.Code, rec.Header().Get("Set-Cookie"), rec.Body.String())
	}
	c, _ := decode(t, rec)["challenge"].(string)
	return c
}

func secondStep(public http.Handler, challenge, code string) *httptest.ResponseRecorder {
	return send(public, http.MethodPost, "/api/v1/sign-in/two-step", "", "", `{"challenge":"`+challenge+`","code":"`+code+`"}`)
}

// Two-step turns on with a first code; after that a password or an emailed link needs a code from the
// app or a recovery code, each good once; it turns off again with a code.
func TestTwoStepSignIn(t *testing.T) {
	t.Parallel()
	trusted, public, mail, now := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", aria, "", "")); body["twoStep"] != false {
		t.Fatalf("before = %v", body)
	}
	if rec := send(public, http.MethodPost, "/api/v1/account/two-step/confirm", aria, "", `{"code":"123456"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("confirm before starting: %d", rec.Code)
	}
	send(public, http.MethodPost, "/api/v1/account/two-step", aria, "", "")
	if rec := send(public, http.MethodPost, "/api/v1/account/two-step/confirm", aria, "", `{"code":"000000"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong first code: %d", rec.Code)
	}
	secret, codes := enableTwoStep(t, public, now, aria)
	if len(codes) != 10 || len(codes[0]) != 11 || codes[0][5] != '-' {
		t.Fatalf("codes = %v", codes)
	}
	if rec := send(public, http.MethodPost, "/api/v1/account/two-step", aria, "", ""); rec.Code != http.StatusConflict {
		t.Fatalf("begin while on: %d", rec.Code)
	}
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", aria, "", "")); body["twoStep"] != true || body["recoveryCodesLeft"] != float64(10) {
		t.Fatalf("after = %v", body)
	}

	c := challenge(t, public, "aria")
	if rec := secondStep(public, c, "000000"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong code: %d", rec.Code)
	}
	if rec := secondStep(public, c, identityapp.TOTP(secret, now.Now())); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the confirming code again: %d", rec.Code)
	}
	now.pass(30 * time.Second)
	rec := secondStep(public, c, identityapp.TOTP(secret, now.Now()))
	if rec.Code != http.StatusOK {
		t.Fatalf("the next code: %d %s", rec.Code, rec.Body.String())
	}
	session(t, rec)
	if rec := secondStep(public, c, identityapp.TOTP(secret, now.Now())); rec.Code != http.StatusGone {
		t.Fatalf("a used challenge: %d", rec.Code)
	}

	c = challenge(t, public, "aria")
	if rec := secondStep(public, c, " "+strings.ToUpper(codes[0])+" "); rec.Code != http.StatusOK {
		t.Fatalf("a recovery code: %d %s", rec.Code, rec.Body.String())
	}
	if rec := secondStep(public, challenge(t, public, "aria"), codes[0]); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a recovery code twice: %d", rec.Code)
	}
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", aria, "", "")); body["recoveryCodesLeft"] != float64(9) {
		t.Fatalf("codes left = %v", body)
	}

	c = challenge(t, public, "aria")
	for range identityapp.ChallengeAttempts {
		secondStep(public, c, "000000")
	}
	now.pass(30 * time.Second)
	if rec := secondStep(public, c, identityapp.TOTP(secret, now.Now())); rec.Code != http.StatusGone {
		t.Fatalf("after five wrong codes: %d", rec.Code)
	}
	c = challenge(t, public, "aria")
	now.pass(identityapp.ChallengeTTL + time.Second)
	if rec := secondStep(public, c, identityapp.TOTP(secret, now.Now())); rec.Code != http.StatusGone {
		t.Fatalf("an expired challenge: %d", rec.Code)
	}

	send(public, http.MethodPost, "/api/v1/sign-in-links", "", "", `{"email":"aria@example.org"}`)
	link := mail.last()
	token := strings.Fields(link[strings.Index(link, "/sign-in-link#")+len("/sign-in-link#"):])[0]
	if rec := send(public, http.MethodPost, "/api/v1/sign-in-links/use", "", "", `{"token":"`+token+`"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("an emailed link with two-step: %d", rec.Code)
	}

	now.pass(30 * time.Second)
	rec = send(public, http.MethodPost, "/api/v1/account/two-step/recovery-codes", aria, "", `{"code":"`+identityapp.TOTP(secret, now.Now())+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("new recovery codes: %d %s", rec.Code, rec.Body.String())
	}
	if rec := secondStep(public, challenge(t, public, "aria"), codes[1]); rec.Code != http.StatusUnauthorized {
		t.Fatalf("an old recovery code: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/account/two-step/disable", aria, "", `{"code":"000000"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("turn off with a wrong code: %d", rec.Code)
	}
	now.pass(30 * time.Second)
	if rec := send(public, http.MethodPost, "/api/v1/account/two-step/disable", aria, "", `{"code":"`+identityapp.TOTP(secret, now.Now())+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("turn off: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"0123456789ab"}`); rec.Code != http.StatusOK {
		t.Fatalf("signing in after: %d", rec.Code)
	}
}

// An Admin Account holds no Admin powers on a password alone; turning two-step on gives the session
// that did it those powers, and so does every sign-in through the second step.
func TestAdminsNeedTwoStep(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	token := invite(t, trusted, `{"hours":1,"admin":true}`)
	rec := send(public, http.MethodPost, "/api/v1/account-invites/accept", "", "", `{"token":"`+token+`","username":"ada","nickname":"Ada","email":"ada@example.org","password":"0123456789ab"}`)
	ada := session(t, rec)
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", ada, "", "")); body["admin"] != true || body["adminPowers"] != false {
		t.Fatalf("a new Admin = %v", body)
	}
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", ada, "", `{"hours":1}`); rec.Code != http.StatusForbidden {
		t.Fatalf("an Admin without two-step invites: %d", rec.Code)
	}
	secret, _ := enableTwoStep(t, public, now, ada)
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", ada, "", `{"hours":1}`); rec.Code != http.StatusCreated {
		t.Fatalf("the session that turned two-step on: %d", rec.Code)
	}
	now.pass(30 * time.Second)
	rec = secondStep(public, challenge(t, public, "ada"), identityapp.TOTP(secret, now.Now()))
	again := session(t, rec)
	if body := decode(t, send(public, http.MethodGet, "/api/v1/account", again, "", "")); body["adminPowers"] != true {
		t.Fatalf("after the second step = %v", body)
	}
	if rec := send(public, http.MethodPost, "/api/v1/admin/account-invites", again, "", `{"hours":1}`); rec.Code != http.StatusCreated {
		t.Fatalf("a two-step session invites: %d", rec.Code)
	}
}
