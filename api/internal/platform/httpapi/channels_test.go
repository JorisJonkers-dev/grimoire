package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

func (o *outbox) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.sent)
}

func (o *outbox) lastHTML() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.html[len(o.html)-1]
}

func prefer(t *testing.T, h http.Handler, session, body string) {
	t.Helper()
	if rec := send(h, http.MethodPut, "/api/v1/notification-preferences", session, "", `{"items":[`+body+`]}`); rec.Code != http.StatusOK {
		t.Fatalf("prefer: %d %s", rec.Code, rec.Body.String())
	}
}

// Each kind reaches devices and email only as the Account's preferences say; email other than security
// waits for a Digest at most hourly, a single Notice going as its own email.
func TestNotificationChannelsFollowPreferences(t *testing.T) {
	t.Parallel()
	st := newStack(t, func(*identityapp.Service) {})
	h, ctx := st.public, context.Background()
	aria, bram, cara := setUp(t, st.trusted, h, "aria"), setUp(t, st.trusted, h, "bram"), setUp(t, st.trusted, h, "cara")
	bramSubject := "account:" + accountID(t, h, bram)
	mails := st.mail.count()

	send(h, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`)
	if got := st.devices.to(bramSubject); len(got) != 1 || got[0] != "aria wants to be Friends https://grimoire.example/friends" {
		t.Fatalf("pushes = %v", got)
	}
	if err := st.social.SendDigests(ctx); err != nil || st.mail.count() != mails {
		t.Fatalf("friend requests are not emailed by default: %v", err)
	}
	send(h, http.MethodPost, "/api/v1/friend-requests/"+firstRequestID(friendsOf(t, h, bram), "incoming")+"/accept", bram, "", "")

	prefer(t, h, bram, `{"kind":"conversation","inApp":true,"push":false,"email":true},{"kind":"friend_request","inApp":true,"push":true,"email":true}`)
	talk := startTalk(t, h, aria, `{"with":["`+accountID(t, h, bram)+`"]}`)
	for _, body := range []string{"First", "Second"} {
		st.now.pass(time.Second)
		send(h, http.MethodPost, "/api/v1/conversations/"+talk+"/messages", aria, "", `{"body":"`+body+`"}`)
	}
	if got := st.devices.to(bramSubject); len(got) != 1 {
		t.Fatalf("a kind with push off is pushed = %v", got)
	}
	if err := st.social.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if st.mail.count() != mails+1 || !strings.HasPrefix(st.mail.last(), "bram@example.org\naria wrote to you\n") || !strings.Contains(st.mail.last(), "Second") ||
		!strings.Contains(st.mail.lastHTML(), "max-width:560px") || !strings.Contains(st.mail.last(), "https://grimoire.example/conversations/"+talk) {
		t.Fatalf("one waiting Notice = %q", st.mail.last())
	}

	send(h, http.MethodPost, "/api/v1/friend-requests", cara, "", `{"username":"bram"}`)
	st.now.pass(time.Second)
	send(h, http.MethodPost, "/api/v1/conversations/"+talk+"/messages", aria, "", `{"body":"Third"}`)
	if err := st.social.SendDigests(ctx); err != nil || st.mail.count() != mails+1 {
		t.Fatalf("a second email within the hour: %v", err)
	}
	st.now.pass(time.Hour)
	if err := st.social.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	digest := st.mail.last()
	if st.mail.count() != mails+2 || !strings.Contains(digest, "What happened in Grimoire") || !strings.Contains(digest, "cara wants to be Friends") || !strings.Contains(digest, "Third") {
		t.Fatalf("the Digest = %q", digest)
	}
	if err := st.social.SendDigests(ctx); err != nil || st.mail.count() != mails+2 {
		t.Fatalf("an empty queue sends nothing: %v", err)
	}

	send(h, http.MethodPut, "/api/v1/account/password", bram, "", `{"password":"another long password"}`)
	if st.mail.count() != mails+3 || !strings.Contains(st.mail.last(), "\nYour password changed\n") || !strings.Contains(st.mail.last(), "cannot be turned off") {
		t.Fatalf("security mail = %q", st.mail.last())
	}
	prefer(t, h, bram, `{"kind":"security","inApp":true,"push":false,"email":false}`)
	send(h, http.MethodPut, "/api/v1/account/password", bram, "", `{"password":"yet another long password"}`)
	if st.mail.count() != mails+3 {
		t.Fatalf("security email turned off is sent: %q", st.mail.last())
	}
}

// A sign-in from a device the Account never used tells its holder, by bell and email.
func TestANewDeviceIsReported(t *testing.T) {
	t.Parallel()
	st := newStack(t, func(*identityapp.Service) {})
	aria := setUp(t, st.trusted, st.public, "aria")
	if got, _ := bell(t, st.public, aria); len(got) != 0 {
		t.Fatalf("the first device = %v", got)
	}
	send(st.public, http.MethodPost, "/api/v1/sign-in", "", "", `{"username":"aria","password":"0123456789ab"}`)
	if got, _ := bell(t, st.public, aria); len(got) != 0 {
		t.Fatalf("a known device = %v", got)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/sign-in", bytes.NewBufferString(`{"username":"aria","password":"0123456789ab"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Safari on a phone")
	st.public.ServeHTTP(httptest.NewRecorder(), req)
	got, _ := bell(t, st.public, aria)
	if len(got) != 1 || got[0]["title"] != "A new sign-in to your Account" || !strings.Contains(got[0]["body"].(string), "Safari on a phone") {
		t.Fatalf("a new device = %v", got)
	}
	if !strings.Contains(st.mail.last(), "A new sign-in to your Account") {
		t.Fatalf("new sign-in mail = %q", st.mail.last())
	}
}
