//go:build integration

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
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
// friendTable is three Accounts: Aria, Bram and Cara.
type friendTable struct {
	trusted, public  http.Handler
	aria, bram, cara string
}

func newFriendTable(t *testing.T) *friendTable {
	t.Helper()
	trusted, public, _, _ := accountServers(t)
	return &friendTable{trusted: trusted, public: public, aria: setUp(t, trusted, public, "aria"), bram: setUp(t, trusted, public, "bram"), cara: setUp(t, trusted, public, "cara")}
}

func (f *friendTable) request(from, to string) *httptest.ResponseRecorder {
	return send(f.public, http.MethodPost, "/api/v1/friend-requests", from, "", `{"username":"`+to+`"}`)
}

// friends has Bram accept Aria's request.
func (f *friendTable) friends(t *testing.T) {
	t.Helper()
	f.request(f.aria, "bram")
	if rec := send(f.public, http.MethodPost, "/api/v1/friend-requests/"+firstRequestID(friendsOf(t, f.public, f.bram), "incoming")+"/accept", f.bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("accept: %d", rec.Code)
	}
}

func TestAFriendRequestGoesOutOnceByAnyCaseOfTheUsername(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	if rec := send(f.public, http.MethodPost, "/api/v1/friend-requests", f.aria, "", `{"username":" BRAM "}`); rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	f.request(f.aria, "bram")
	if got := usernames(friendsOf(t, f.public, f.aria), "outgoing"); strings.Join(got, ",") != "bram" {
		t.Fatalf("outgoing = %v", got)
	}
	if got := usernames(friendsOf(t, f.public, f.aria), "friends"); len(got) != 0 {
		t.Fatalf("friends before accepting = %v", got)
	}
}

func TestTheReceiverSeesTheIncomingRequest(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.request(f.aria, "bram")
	if got := usernames(friendsOf(t, f.public, f.bram), "incoming"); strings.Join(got, ",") != "aria" {
		t.Fatalf("incoming = %v", got)
	}
}

func TestTheSenderCannotAcceptTheirOwnRequest(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.request(f.aria, "bram")
	if rec := send(f.public, http.MethodPost, "/api/v1/friend-requests/"+firstRequestID(friendsOf(t, f.public, f.bram), "incoming")+"/accept", f.aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAnAcceptedRequestMakesBothFriends(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.friends(t)
	for name, s := range map[string]string{"aria": f.aria, "bram": f.bram} {
		t.Run(name, func(t *testing.T) {
			page := friendsOf(t, f.public, s)
			if len(page["friends"].([]any)) != 1 || len(page["incoming"].([]any))+len(page["outgoing"].([]any)) != 0 {
				t.Fatalf("page = %v", page)
			}
		})
	}
}

func TestFriendsCannotBeRequestedAgain(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.friends(t)
	if rec := f.request(f.bram, "aria"); rec.Code != http.StatusConflict {
		t.Fatalf("%d", rec.Code)
	}
}

func TestADeclinedRequestBefriendsNobody(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.request(f.cara, "aria")
	if rec := send(f.public, http.MethodPost, "/api/v1/friend-requests/"+firstRequestID(friendsOf(t, f.public, f.aria), "incoming")+"/decline", f.aria, "", `{}`); rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d", rec.Code)
	}
	if got := usernames(friendsOf(t, f.public, f.cara), "friends"); len(got) != 0 {
		t.Fatalf("friends = %v", got)
	}
}

func TestACrossingRequestIsTakenAsAccepting(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.request(f.cara, "aria")
	if rec := f.request(f.aria, "cara"); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	if got := usernames(friendsOf(t, f.public, f.cara), "friends"); strings.Join(got, ",") != "aria" {
		t.Fatalf("friends = %v", got)
	}
}

func TestAFriendIsUnfriendedOnce(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.friends(t)
	ariaID := accountID(t, f.public, f.aria)
	if rec := send(f.public, http.MethodDelete, "/api/v1/friends/"+ariaID, f.bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unfriend: %d", rec.Code)
	}
	if rec := send(f.public, http.MethodDelete, "/api/v1/friends/"+ariaID, f.bram, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("twice: %d", rec.Code)
	}
}

func TestOnlyTheSenderWithdrawsARequest(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	f.request(f.bram, "aria")
	mine := firstRequestID(friendsOf(t, f.public, f.bram), "outgoing")
	if rec := send(f.public, http.MethodDelete, "/api/v1/friend-requests/"+mine, f.aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else: %d", rec.Code)
	}
	if rec := send(f.public, http.MethodDelete, "/api/v1/friend-requests/"+mine, f.bram, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("withdraw: %d", rec.Code)
	}
}

func TestNobodyRequestsThemselfOrAnAccountThatIsNotThere(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	for to, want := range map[string]int{"aria": http.StatusUnprocessableEntity, "nobody": http.StatusNotFound} {
		t.Run(to, func(t *testing.T) {
			if rec := f.request(f.aria, to); rec.Code != want {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestWithoutAnAccountThereAreNoFriends(t *testing.T) {
	t.Parallel()
	f := newFriendTable(t)
	if rec := send(f.trusted, http.MethodGet, "/api/v1/friends", "", "estate-without-account", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
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
