package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func befriend(t *testing.T, h http.Handler, a, b, bName string) {
	t.Helper()
	send(h, http.MethodPost, "/api/v1/friend-requests", a, "", `{"username":"`+bName+`"}`)
	id := firstRequestID(friendsOf(t, h, b), "incoming")
	if rec := send(h, http.MethodPost, "/api/v1/friend-requests/"+id+"/accept", b, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("befriend: %d", rec.Code)
	}
}

func startTalk(t *testing.T, h http.Handler, session, body string) string {
	t.Helper()
	rec := send(h, http.MethodPost, "/api/v1/conversations", session, "", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := decode(t, rec)["id"].(string)
	return id
}

func messages(t *testing.T, h http.Handler, session, conversation string) []any {
	t.Helper()
	rec := send(h, http.MethodGet, "/api/v1/conversations/"+conversation+"/messages", session, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("messages: %d %s", rec.Code, rec.Body.String())
	}
	items, _ := decode(t, rec)["items"].([]any)
	return items
}

func mentionOf(msg any) map[string]any {
	ms, _ := msg.(map[string]any)["mentions"].([]any)
	m, _ := ms[0].(map[string]any)
	return m
}

func unread(t *testing.T, h http.Handler, session, conversation string) float64 {
	t.Helper()
	for _, c := range decode(t, send(h, http.MethodGet, "/api/v1/conversations", session, "", ""))["items"].([]any) {
		if entry, _ := c.(map[string]any); entry["id"] == conversation {
			n, _ := entry["unread"].(float64)
			return n
		}
	}
	t.Fatalf("no conversation %s", conversation)
	return 0
}

// Friends talk one-to-one or in a group; only members read a Conversation, each Mention opens only for
// a reader who may open it, and every member has an unread count.
func TestConversations(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	aria, bram, cara, dana := setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram"), setUp(t, trusted, public, "cara"), setUp(t, trusted, public, "dana")
	befriend(t, public, aria, bram, "bram")
	befriend(t, public, aria, dana, "dana")
	ariaID, bramID, caraID, danaID := accountID(t, public, aria), accountID(t, public, bram), accountID(t, public, cara), accountID(t, public, dana)

	if rec := send(public, http.MethodPost, "/api/v1/conversations", aria, "", `{"with":["`+caraID+`"]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("talking with someone not a Friend: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/conversations", aria, "", `{"with":["`+ariaID+`"]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("talking with yourself: %d", rec.Code)
	}
	direct := startTalk(t, public, aria, `{"with":["`+bramID+`"]}`)
	if again := startTalk(t, public, bram, `{"with":["`+ariaID+`"]}`); again != direct {
		t.Fatalf("the one-to-one Conversation is found again: %s, %s", direct, again)
	}

	rec := send(public, http.MethodPost, "/api/v1/campaigns", aria, "", `{"name":"Morvain","displayName":"Aria"}`)
	campaign, _ := decode(t, rec)["id"].(string)
	token, _ := decode(t, send(public, http.MethodPost, "/api/v1/campaigns/"+campaign+"/invites", aria, "", ""))["token"].(string)
	send(public, http.MethodPost, "/api/v1/invites/accept", bram, "", `{"token":"`+token+`","displayName":"Bram"}`)
	rec = send(public, http.MethodPost, "/api/v1/campaigns/"+campaign+"/characters", bram, "", build)
	kara, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated {
		t.Fatalf("character: %d %s", rec.Code, rec.Body.String())
	}
	if found := send(public, http.MethodGet, "/api/v1/mentionables?q=KAR", bram, "", "").Body.String(); !strings.Contains(found, `"name":"Kara"`) || !strings.Contains(found, `"campaignName":"Morvain"`) {
		t.Fatalf("mentionables = %s", found)
	}
	mention := `"mentions":[{"kind":"character","campaignId":"` + campaign + `","id":"` + kara + `"}]`
	now.pass(time.Minute)
	rec = send(public, http.MethodPost, "/api/v1/conversations/"+direct+"/messages", bram, "", `{"body":"  Meet @Kara  ",`+mention+`}`)
	if rec.Code != http.StatusCreated || decode(t, rec)["body"] != "Meet @Kara" {
		t.Fatalf("send: %d %s", rec.Code, rec.Body.String())
	}
	if n := unread(t, public, aria, direct); n != 1 {
		t.Fatalf("aria's unread = %v", n)
	}
	got := messages(t, public, aria, direct)
	if m := mentionOf(got[0]); len(got) != 1 || m["open"] != true || m["label"] != "Kara" {
		t.Fatalf("aria reads = %v", got)
	}
	if n := unread(t, public, aria, direct); n != 0 {
		t.Fatalf("after reading = %v", n)
	}
	if n := unread(t, public, bram, direct); n != 0 {
		t.Fatalf("one's own message is never unread = %v", n)
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/conversations/" + direct + "/messages", ""},
		{http.MethodPost, "/api/v1/conversations/" + direct + "/messages", `{"body":"hi"}`},
	} {
		if rec := send(public, c.method, c.path, cara, "", c.body); rec.Code != http.StatusNotFound {
			t.Errorf("a non-member %s: %d", c.method, rec.Code)
		}
	}
	if list := decode(t, send(public, http.MethodGet, "/api/v1/conversations", cara, "", ""))["items"].([]any); len(list) != 0 {
		t.Fatalf("cara's Conversations = %v", list)
	}

	now.pass(time.Minute)
	group := startTalk(t, public, aria, `{"with":["`+bramID+`","`+danaID+`"],"title":" Party "}`)
	send(public, http.MethodPost, "/api/v1/conversations/"+group+"/messages", aria, "", `{"body":"Who brings @Kara?",`+mention+`}`)
	hidden := mentionOf(messages(t, public, dana, group)[0])
	if hidden["open"] != false || hidden["label"] != nil {
		t.Fatalf("dana, outside the Campaign, sees = %v", hidden)
	}
	if rec := send(public, http.MethodPost, "/api/v1/conversations/"+group+"/messages", dana, "", `{"body":"Me? @Kara",`+mention+`}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("mentioning what you cannot open: %d", rec.Code)
	}
	if rec := send(public, http.MethodPost, "/api/v1/conversations/"+group+"/messages", dana, "", `{"body":"   "}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an empty message: %d", rec.Code)
	}
	list := send(public, http.MethodGet, "/api/v1/conversations", bram, "", "").Body.String()
	if !strings.Contains(list, `"title":"Party"`) || !strings.Contains(list, `"lastBody":"Who brings @Kara?"`) {
		t.Fatalf("bram's list = %s", list)
	}
	page := send(public, http.MethodGet, "/api/v1/conversations/"+group+"/messages?before=2000-01-01T00:00:00Z", dana, "", "")
	if items, _ := decode(t, page)["items"].([]any); len(items) != 0 {
		t.Fatalf("an older page = %v", items)
	}
}
