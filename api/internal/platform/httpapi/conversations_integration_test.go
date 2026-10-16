//go:build integration

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
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
// talkTable is Aria with two Friends, Bram and Dana, and Cara, who is no Friend, and the one-to-one
// Conversation between Aria and Bram.
type talkTable struct {
	trusted, public                http.Handler
	now                            *clock
	aria, bram, cara, dana         string
	ariaID, bramID, caraID, danaID string
	direct                         string
}

func newTalkTable(t *testing.T) *talkTable {
	t.Helper()
	trusted, public, _, now := accountServers(t)
	c := &talkTable{trusted: trusted, public: public, now: now}
	c.aria, c.bram, c.cara, c.dana = setUp(t, trusted, public, "aria"), setUp(t, trusted, public, "bram"), setUp(t, trusted, public, "cara"), setUp(t, trusted, public, "dana")
	befriend(t, public, c.aria, c.bram, "bram")
	befriend(t, public, c.aria, c.dana, "dana")
	c.ariaID, c.bramID, c.caraID, c.danaID = accountID(t, public, c.aria), accountID(t, public, c.bram), accountID(t, public, c.cara), accountID(t, public, c.dana)
	c.direct = startTalk(t, public, c.aria, `{"with":["`+c.bramID+`"]}`)
	return c
}

// withKara puts Bram's Character Kara in Aria's Campaign Morvain, and returns a mention of her.
func (c *talkTable) withKara(t *testing.T) string {
	t.Helper()
	campaign, _ := decode(t, send(c.public, http.MethodPost, "/api/v1/campaigns", c.aria, "", `{"name":"Morvain","displayName":"Aria"}`))["id"].(string)
	token, _ := decode(t, send(c.public, http.MethodPost, "/api/v1/campaigns/"+campaign+"/invites", c.aria, "", ""))["token"].(string)
	send(c.public, http.MethodPost, "/api/v1/invites/accept", c.bram, "", `{"token":"`+token+`","displayName":"Bram"}`)
	rec := send(c.public, http.MethodPost, "/api/v1/campaigns/"+campaign+"/characters", c.bram, "", build)
	if rec.Code != http.StatusCreated {
		t.Fatalf("character: %d %s", rec.Code, rec.Body.String())
	}
	kara, _ := decode(t, rec)["id"].(string)
	return `"mentions":[{"kind":"character","campaignId":"` + campaign + `","id":"` + kara + `"}]`
}

// karaMet has Bram tell Aria to meet Kara, with a mention of her.
func (c *talkTable) karaMet(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	mention := c.withKara(t)
	c.now.pass(time.Minute)
	return send(c.public, http.MethodPost, "/api/v1/conversations/"+c.direct+"/messages", c.bram, "", `{"body":"  Meet @Kara  ",`+mention+`}`)
}

// party starts a group Conversation of Aria, Bram and Dana with a message mentioning Kara.
func (c *talkTable) party(t *testing.T) (string, string) {
	t.Helper()
	mention := c.withKara(t)
	c.now.pass(time.Minute)
	group := startTalk(t, c.public, c.aria, `{"with":["`+c.bramID+`","`+c.danaID+`"],"title":" Party "}`)
	send(c.public, http.MethodPost, "/api/v1/conversations/"+group+"/messages", c.aria, "", `{"body":"Who brings @Kara?",`+mention+`}`)
	return group, mention
}

func TestAConversationIsOnlyWithAFriend(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	if rec := send(c.public, http.MethodPost, "/api/v1/conversations", c.aria, "", `{"with":["`+c.caraID+`"]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}

func TestNobodyStartsAConversationWithThemself(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	if rec := send(c.public, http.MethodPost, "/api/v1/conversations", c.aria, "", `{"with":["`+c.ariaID+`"]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestTheOneToOneConversationIsFoundAgainFromEitherSide(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	if again := startTalk(t, c.public, c.bram, `{"with":["`+c.ariaID+`"]}`); again != c.direct {
		t.Fatalf("%s, then %s", c.direct, again)
	}
}

func TestACharacterIsFoundToMentionByPartOfItsName(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	c.withKara(t)
	if found := send(c.public, http.MethodGet, "/api/v1/mentionables?q=KAR", c.bram, "", "").Body.String(); !strings.Contains(found, `"name":"Kara"`) || !strings.Contains(found, `"campaignName":"Morvain"`) {
		t.Fatalf("mentionables = %s", found)
	}
}

func TestASentMessageIsTrimmed(t *testing.T) {
	t.Parallel()
	if rec := newTalkTable(t).karaMet(t); rec.Code != http.StatusCreated || decode(t, rec)["body"] != "Meet @Kara" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestAMessageIsUnreadUntilReadAndOnesOwnNeverIs(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	c.karaMet(t)
	if n := unread(t, c.public, c.aria, c.direct); n != 1 {
		t.Fatalf("aria's unread = %v", n)
	}
	messages(t, c.public, c.aria, c.direct)
	if n := unread(t, c.public, c.aria, c.direct); n != 0 {
		t.Fatalf("after reading = %v", n)
	}
	if n := unread(t, c.public, c.bram, c.direct); n != 0 {
		t.Fatalf("bram's own = %v", n)
	}
}

func TestAMentionOpensForAReaderWhoMayOpenIt(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	c.karaMet(t)
	if got := messages(t, c.public, c.aria, c.direct); len(got) != 1 || mentionOf(got[0])["open"] != true || mentionOf(got[0])["label"] != "Kara" {
		t.Fatalf("aria reads %v", got)
	}
}

func TestSomeoneOutsideAConversationCannotReadOrWriteIt(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	for _, r := range []struct{ method, body string }{{http.MethodGet, ""}, {http.MethodPost, `{"body":"hi"}`}} {
		t.Run(r.method, func(t *testing.T) {
			if rec := send(c.public, r.method, "/api/v1/conversations/"+c.direct+"/messages", c.cara, "", r.body); rec.Code != http.StatusNotFound {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
	if list := decode(t, send(c.public, http.MethodGet, "/api/v1/conversations", c.cara, "", ""))["items"].([]any); len(list) != 0 {
		t.Fatalf("cara's Conversations = %v", list)
	}
}

func TestAMentionStaysClosedToAReaderOutsideTheCampaign(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	group, _ := c.party(t)
	if hidden := mentionOf(messages(t, c.public, c.dana, group)[0]); hidden["open"] != false || hidden["label"] != nil {
		t.Fatalf("dana sees %v", hidden)
	}
}

func TestNobodyMentionsWhatTheyCannotOpen(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	group, mention := c.party(t)
	if rec := send(c.public, http.MethodPost, "/api/v1/conversations/"+group+"/messages", c.dana, "", `{"body":"Me? @Kara",`+mention+`}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAnEmptyMessageIsRefused(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	if rec := send(c.public, http.MethodPost, "/api/v1/conversations/"+c.direct+"/messages", c.bram, "", `{"body":"   "}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestTheConversationListShowsEachTitleAndLastMessage(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	c.party(t)
	if list := send(c.public, http.MethodGet, "/api/v1/conversations", c.bram, "", "").Body.String(); !strings.Contains(list, `"title":"Party"`) || !strings.Contains(list, `"lastBody":"Who brings @Kara?"`) {
		t.Fatalf("bram's list = %s", list)
	}
}

func TestAPageOlderThanEveryMessageIsEmpty(t *testing.T) {
	t.Parallel()
	c := newTalkTable(t)
	group, _ := c.party(t)
	page := send(c.public, http.MethodGet, "/api/v1/conversations/"+group+"/messages?before=2000-01-01T00:00:00Z", c.dana, "", "")
	if items, _ := decode(t, page)["items"].([]any); len(items) != 0 {
		t.Fatalf("page = %v", items)
	}
}
