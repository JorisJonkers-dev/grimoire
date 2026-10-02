package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func friendsOf(t *testing.T, h http.Handler, session string) map[string]any {
	t.Helper()
	rec := send(h, http.MethodGet, "/api/v1/friends", session, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("friends: %d %s", rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

func usernames(page map[string]any, key string) []string {
	var out []string
	for _, e := range page[key].([]any) {
		person, _ := e.(map[string]any)["person"].(map[string]any)
		out = append(out, person["username"].(string))
	}
	return out
}

func firstRequestID(page map[string]any, key string) string {
	id, _ := page[key].([]any)[0].(map[string]any)["id"].(string)
	return id
}

// A Friend request by Username becomes a mutual friendship only once accepted; declining keeps the two
// apart, and a request crossing one already sent is accepted at once.
func TestFriendRequests(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	aria, bram, cara := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram"), setUp(t, trusted, public, "cara")
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":" BRAM "}`); rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`)
	if got := usernames(friendsOf(t, public, aria), "outgoing"); strings.Join(got, ",") != "bram" {
		t.Fatalf("outgoing = %v", got)
	}
	if got := usernames(friendsOf(t, public, aria), "friends"); len(got) != 0 {
		t.Fatalf("friends before accepting = %v", got)
	}
	page := friendsOf(t, public, bram)
	if got := usernames(page, "incoming"); strings.Join(got, ",") != "aria" {
		t.Fatalf("incoming = %v", got)
	}
	request := firstRequestID(page, "incoming")
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests/"+request+"/accept", aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the sender accepts their own request: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests/"+request+"/accept", bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("accept: %d", rec.Code)
	}
	for _, s := range []string{aria, bram} {
		page := friendsOf(t, public, s)
		if len(page["friends"].([]any)) != 1 || len(page["incoming"].([]any))+len(page["outgoing"].([]any)) != 0 {
			t.Fatalf("after accepting = %v", page)
		}
	}
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests", bram, "", `{"username":"aria"}`); rec.Code != http.StatusConflict {
		t.Fatalf("already Friends: %d", rec.Code)
	}

	send(public, http.MethodPost, "/api/v1/friend-requests", cara, "", `{"username":"aria"}`)
	declined := firstRequestID(friendsOf(t, public, aria), "incoming")
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests/"+declined+"/decline", aria, "", `{}`); rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d", rec.Code)
	}
	if got := usernames(friendsOf(t, public, cara), "friends"); len(got) != 0 {
		t.Fatalf("a declined request befriends = %v", got)
	}
	send(public, http.MethodPost, "/api/v1/friend-requests", cara, "", `{"username":"aria"}`)
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"cara"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("a crossing request: %d", rec.Code)
	}
	if got := usernames(friendsOf(t, public, cara), "friends"); strings.Join(got, ",") != "aria" {
		t.Fatalf("a crossing request accepts = %v", got)
	}

	ariaID := accountID(t, public, aria)
	if rec := send(public, http.MethodDelete, "/api/v1/friends/"+ariaID, bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unfriend: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/friends/"+ariaID, bram, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unfriend twice: %d", rec.Code)
	}
	send(public, http.MethodPost, "/api/v1/friend-requests", bram, "", `{"username":"aria"}`)
	mine := firstRequestID(friendsOf(t, public, bram), "outgoing")
	if rec := send(public, http.MethodDelete, "/api/v1/friend-requests/"+mine, aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("withdrawing someone else's request: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/friend-requests/"+mine, bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("withdraw: %d", rec.Code)
	}
	for body, want := range map[string]int{`{"username":"aria"}`: http.StatusUnprocessableEntity, `{"username":"nobody"}`: http.StatusNotFound} {
		if rec := send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", body); rec.Code != want {
			t.Errorf("request %s = %d", body, rec.Code)
		}
	}
	if rec := send(trusted, http.MethodGet, "/api/v1/friends", "", "estate-without-account", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("no Account: %d", rec.Code)
	}
}

// Declining with a block hides every later request from that sender, who cannot tell; unblocking lets
// them through again.
func TestBlockingHidesRequests(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	aria, bram := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram")
	send(public, http.MethodPost, "/api/v1/friend-requests", bram, "", `{"username":"aria"}`)
	request := firstRequestID(friendsOf(t, public, aria), "incoming")
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests/"+request+"/decline", aria, "", `{"block":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("decline and block: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests", bram, "", `{"username":"aria"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("a blocked sender's request answers the same: %d", rec.Code)
	}
	page := friendsOf(t, public, aria)
	if len(page["incoming"].([]any)) != 0 || strings.Join(usernames(page, "blocked"), ",") != "bram" {
		t.Fatalf("after blocking = %v", page)
	}
	if got := usernames(friendsOf(t, public, bram), "outgoing"); strings.Join(got, ",") != "aria" {
		t.Fatalf("the sender still sees it pending = %v", got)
	}
	if rec := send(public, http.MethodPost, "/api/v1/friend-requests", aria, "", `{"username":"bram"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("a request to the blocked: %d", rec.Code)
	}
	if got := usernames(friendsOf(t, public, aria), "friends"); len(got) != 0 {
		t.Fatalf("a blocked request is never accepted by crossing = %v", got)
	}
	bramID := accountID(t, public, bram)
	if rec := send(public, http.MethodDelete, "/api/v1/blocks/"+bramID, aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unblock: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/blocks/"+bramID, aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unblock twice: %d", rec.Code)
	}
	if got := usernames(friendsOf(t, public, aria), "incoming"); strings.Join(got, ",") != "bram" {
		t.Fatalf("after unblocking = %v", got)
	}
}
