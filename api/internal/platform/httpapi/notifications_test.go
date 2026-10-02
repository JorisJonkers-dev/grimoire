package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func bell(t *testing.T, h http.Handler, session string) ([]map[string]any, float64) {
	t.Helper()
	rec := send(h, http.MethodGet, "/api/v1/notifications", session, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bell: %d %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	var out []map[string]any
	for _, n := range body["items"].([]any) {
		out = append(out, n.(map[string]any))
	}
	unread, _ := body["unread"].(float64)
	return out, unread
}

// Friend requests, acceptances, Conversation messages and sign-in changes reach the bell, each with
// one action; a busy Conversation shows once until read.
func TestNotificationsAreDelivered(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	aria, bram := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram")
	send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`)
	got, unread := bell(t, public, bram)
	if unread != 1 || got[0]["kind"] != "friend_request" || got[0]["title"] != "aria wants to be Friends" || got[0]["actionLabel"] != "Review" || got[0]["actionPath"] != "/friends" {
		t.Fatalf("bram's bell = %v %v", got, unread)
	}
	send(public, http.MethodPost, "/api/v1/friend-requests/"+firstRequestID(friendsOf(t, public, bram), "incoming")+"/accept", bram, "", "")
	if got, _ := bell(t, public, aria); len(got) != 1 || got[0]["title"] != "bram accepted your Friend request" {
		t.Fatalf("aria's bell = %v", got)
	}

	talk := startTalk(t, public, aria, `{"with":["`+accountID(t, public, bram)+`"]}`)
	for _, body := range []string{"First", "Second"} {
		now.pass(time.Second)
		send(public, http.MethodPost, "/api/v1/conversations/"+talk+"/messages", aria, "", `{"body":"`+body+`"}`)
	}
	got, _ = bell(t, public, bram)
	var talks []map[string]any
	for _, n := range got {
		if n["kind"] == "conversation" {
			talks = append(talks, n)
		}
	}
	if len(talks) != 1 || talks[0]["body"] != "Second" || talks[0]["actionPath"] != "/conversations/"+talk || talks[0]["title"] != "aria wrote to you" {
		t.Fatalf("conversation notices = %v", talks)
	}
	if got, _ := bell(t, public, aria); len(got) != 1 {
		t.Fatalf("one's own messages never ring = %v", got)
	}

	send(public, http.MethodPut, "/api/v1/account/password", aria, "", `{"password":"another long password"}`)
	got, _ = bell(t, public, aria)
	if got[0]["kind"] != "security" || got[0]["title"] != "Your password changed" || got[0]["actionPath"] != "/account" {
		t.Fatalf("security notice = %v", got[0])
	}

	id, _ := got[0]["id"].(string)
	if rec := send(public, http.MethodPost, "/api/v1/notifications/"+id+"/read", bram, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("reading someone else's: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/notifications/"+id+"/read", aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("read: %d", rec.Code)
	}
	if got, unread := bell(t, public, aria); got[0]["read"] != true || unread != 1 {
		t.Fatalf("after reading one = %v %v", got, unread)
	}
	if rec := send(public, http.MethodPost, "/api/v1/notifications/"+id+"/read", aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("reading twice: %d", rec.Code)
	}
	send(public, http.MethodPost, "/api/v1/notifications/read", aria, "", "")
	if _, unread := bell(t, public, aria); unread != 0 {
		t.Fatalf("after reading all = %v", unread)
	}
	now.pass(time.Second)
	send(public, http.MethodPost, "/api/v1/conversations/"+talk+"/messages", bram, "", `{"body":"Third"}`)
	if got, unread := bell(t, public, aria); unread != 1 || got[0]["body"] != "Third" {
		t.Fatalf("a new message after reading = %v %v", got, unread)
	}
}

// Preferences start from the defaults, turning a kind off in app keeps it out of the bell, and
// security always shows.
func TestNotificationPreferences(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	aria, bram := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram")
	rec := send(public, http.MethodGet, "/api/v1/notification-preferences", bram, "", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `{"kind":"security","inApp":true,"push":true,"email":true}`) ||
		!strings.Contains(body, `{"kind":"release_note","inApp":true,"push":false,"email":false}`) || len(decode(t, rec)["items"].([]any)) != 8 {
		t.Fatalf("defaults: %d %s", rec.Code, body)
	}
	rec = send(public, http.MethodPut, "/api/v1/notification-preferences", bram, "", `{"items":[{"kind":"friend_request","inApp":false,"push":false,"email":true}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `{"kind":"friend_request","inApp":false,"push":false,"email":true}`) {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`)
	if got, unread := bell(t, public, bram); len(got) != 0 || unread != 0 {
		t.Fatalf("a kind turned off rings = %v", got)
	}
	if got := usernames(friendsOf(t, public, bram), "incoming"); strings.Join(got, ",") != "aria" {
		t.Fatalf("the request itself still arrives = %v", got)
	}
	if rec := send(public, http.MethodPut, "/api/v1/notification-preferences", bram, "", `{"items":[{"kind":"security","inApp":false,"push":true,"email":true}]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("security off in app: %d", rec.Code)
	}
}
