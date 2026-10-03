package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func libraryStack(t *testing.T) http.Handler {
	t.Helper()
	return libraryStackWith(t, nil)
}

// notices records the Proposal Notifications rung, by subject.
type notices struct {
	mu   sync.Mutex
	rung map[string][]libraryapp.Notice
}

func (n *notices) Notify(_ context.Context, subject string, note libraryapp.Notice) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.rung == nil {
		n.rung = map[string][]libraryapp.Notice{}
	}
	n.rung[subject] = append(n.rung[subject], note)
	return nil
}

func (n *notices) last(subject string) libraryapp.Notice {
	n.mu.Lock()
	defer n.mu.Unlock()
	list := n.rung[subject]
	if len(list) == 0 {
		return libraryapp.Notice{}
	}
	return list[len(list)-1]
}

func libraryStackWith(t *testing.T, bell libraryapp.Notifier) http.Handler {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	repo := campaignpg.New(store.Pool())
	lib := &libraryapp.Service{
		Repo: librarypg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now, Notices: bell, Log: quiet,
		Admins: admins{"admin": true}, Surfaces: playpg.New(store.Pool()).SurfaceKinds,
	}
	return campaignServer(t, app.NewService(repo), httpapi.LibraryService(lib))
}

func fieldsOf(t *testing.T, v any) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range v.([]any) {
		m := f.(map[string]any)
		out[m["name"].(string)] = m["value"].(string)
	}
	return out
}

// A DM keeps a creature in their Library, links it into two Campaigns, overrides its hit points in one
// and pins the other to its first Revision; a later edit reaches only the Campaign that follows it.
func TestLibraryEntriesLinkOverrideAndPin(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	morvain, _ := campaignWithPlayer(t, h)
	rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"Second","displayName":"Joris"}`)
	second := decode(t, rec)["id"].(string)

	rec = call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"52"},{"name":"AC","value":"17"}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	hag := decode(t, rec)
	id := hag["id"].(string)
	if hag["revision"] != float64(1) || fieldsOf(t, hag["fields"])["HP"] != "52" {
		t.Fatalf("the new entry = %v", hag)
	}
	for body, want := range map[string]int{
		`{"kind":"vehicle","name":"Cart","fields":[]}`:                    http.StatusBadRequest,
		`{"kind":"npc","name":"   ","fields":[]}`:                         http.StatusUnprocessableEntity,
		`{"kind":"npc","name":"Odo","fields":[{"name":" ","value":"x"}]}`: http.StatusUnprocessableEntity,
	} {
		if rec := call(h, http.MethodPost, "/api/v1/library", "dm", body); rec.Code != want {
			t.Fatalf("%s: %d", body, rec.Code)
		}
	}
	call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`)
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/library?kind=creature", "dm", "")); len(list) != 1 || list[0]["name"] != "Bog Hag" {
		t.Fatalf("only creatures = %v", list)
	}
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/library", "player", "")); len(list) != 0 {
		t.Fatalf("a Library is its owner's own = %v", list)
	}
	if rec := call(h, http.MethodGet, "/api/v1/library/"+id, "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("another account's entry: %d", rec.Code)
	}

	link := func(campaign, who string) *httptest.ResponseRecorder {
		return call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", who, `{"entryId":"`+id+`"}`)
	}
	if rec := link(morvain, "player"); rec.Code != http.StatusForbidden {
		t.Fatalf("a player links: %d", rec.Code)
	}
	for _, c := range []string{morvain, second, second} {
		if rec := link(c, "dm"); rec.Code != http.StatusOK {
			t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
		}
	}
	base := "/api/v1/campaigns/" + morvain + "/library/" + id
	rec = call(h, http.MethodPut, base+"/override", "dm", `{"fields":[{"name":"HP","value":"30"},{"name":"Mood","value":"wounded"}]}`)
	over := decode(t, rec)
	if f := fieldsOf(t, over["fields"]); rec.Code != http.StatusOK || f["HP"] != "30" || f["AC"] != "17" || f["Mood"] != "wounded" || fieldsOf(t, over["base"])["HP"] != "52" {
		t.Fatalf("the Campaign Override wins over the base = %d %v", rec.Code, over)
	}
	pin := "/api/v1/campaigns/" + second + "/library/" + id + "/pin"
	if rec := call(h, http.MethodPut, pin, "dm", `{"revision":2}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("pinning a Revision it has not got: %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, pin, "dm", `{"revision":1}`); rec.Code != http.StatusOK || decode(t, rec)["pinnedRevision"] != float64(1) {
		t.Fatalf("pin: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPut, "/api/v1/library/"+id, "dm", `{"name":"Bog Hag Matriarch","fields":[{"name":"HP","value":"80"},{"name":"AC","value":"18"}]}`)
	edited := decode(t, rec)
	if rec.Code != http.StatusOK || edited["entry"].(map[string]any)["revision"] != float64(2) || len(edited["revisions"].([]any)) != 2 || len(edited["uses"].([]any)) != 2 {
		t.Fatalf("the edit = %d %v", rec.Code, edited)
	}
	resolved := func(campaign string) map[string]any {
		t.Helper()
		linked := decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/library", "dm", ""))
		if len(linked) != 1 {
			t.Fatalf("linked in %s = %v", campaign, linked)
		}
		return linked[0]
	}
	if m := resolved(morvain); fieldsOf(t, m["fields"])["AC"] != "18" || fieldsOf(t, m["fields"])["HP"] != "30" || m["baseName"] != "Bog Hag Matriarch" {
		t.Fatalf("the following Campaign sees the edit under its override = %v", m)
	}
	if m := resolved(second); fieldsOf(t, m["fields"])["AC"] != "17" || m["baseName"] != "Bog Hag" {
		t.Fatalf("the pinned Campaign ignores the edit = %v", m)
	}
	if rec := call(h, http.MethodDelete, pin, "dm", ""); rec.Code != http.StatusOK || fieldsOf(t, decode(t, rec)["fields"])["AC"] != "18" {
		t.Fatalf("unpinned, it follows the latest: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+morvain+"/library", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a player reads the linked entries: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, base, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unlink: %d", rec.Code)
	}
	for _, r := range []*httptest.ResponseRecorder{
		call(h, http.MethodDelete, base, "dm", ""),
		call(h, http.MethodPut, base+"/override", "dm", `{"fields":[]}`),
		call(h, http.MethodDelete, base+"/pin", "dm", ""),
	} {
		if r.Code != http.StatusNotFound {
			t.Fatalf("a change to an unlinked entry: %d", r.Code)
		}
	}
	if rec := call(h, http.MethodPut, "/api/v1/library/"+id, "player", `{"name":"Mine","fields":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("editing another account's entry: %d", rec.Code)
	}
}

// A DM groups entries into a Collection and switches it on per Campaign: on brings its entries in with
// their overrides, off hides them unless the DM linked one directly.
func TestCollectionsSwitchedOnPerCampaign(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	entry := func(who, name string) string {
		t.Helper()
		rec := call(h, http.MethodPost, "/api/v1/library", who, `{"kind":"spell","name":"`+name+`","fields":[{"name":"Level","value":"1"}]}`)
		return decode(t, rec)["id"].(string)
	}
	moon, sprite, theirs := entry("dm", "Moonbeam Ward"), entry("dm", "Sprite Call"), entry("player", "Borrowed")
	rec := call(h, http.MethodPost, "/api/v1/library/collections", "dm", `{"name":"Feywild","description":"Fey magic"}`)
	fey := decode(t, rec)
	id := fey["id"].(string)
	if rec.Code != http.StatusCreated || fey["mine"] != true || len(fey["entryIds"].([]any)) != 0 {
		t.Fatalf("the new Collection = %d %v", rec.Code, fey)
	}
	if rec := call(h, http.MethodPost, "/api/v1/library/collections", "dm", `{"name":"   "}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a nameless Collection: %d", rec.Code)
	}
	put := func(who, body string) *httptest.ResponseRecorder {
		return call(h, http.MethodPut, "/api/v1/library/collections/"+id, who, body)
	}
	if rec := put("dm", `{"name":"Feywild","entryIds":["`+theirs+`"]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("another account's entry in a Collection: %d", rec.Code)
	}
	if rec := put("player", `{"name":"Mine","entryIds":[]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("editing another account's Collection: %d", rec.Code)
	}
	if rec := put("dm", `{"name":"Feywild Expansion","description":"Fey","entryIds":["`+moon+`","`+sprite+`"]}`); rec.Code != http.StatusOK || len(decode(t, rec)["entryIds"].([]any)) != 2 {
		t.Fatalf("filling the Collection: %d %s", rec.Code, rec.Body.String())
	}
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/library/collections", "dm", "")); len(list) != 1 || list[0]["name"] != "Feywild Expansion" {
		t.Fatalf("my Collections = %v", list)
	}

	names := func() []string {
		t.Helper()
		var out []string
		for _, l := range decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/library", "dm", "")) {
			out = append(out, l["entry"].(map[string]any)["name"].(string))
		}
		return out
	}
	switchTo := func(who, on string) *httptest.ResponseRecorder {
		return call(h, http.MethodPut, "/api/v1/campaigns/"+campaign+"/collections/"+id, who, `{"on":`+on+`}`)
	}
	if rec := switchTo("player", "true"); rec.Code != http.StatusForbidden {
		t.Fatalf("a player switches a Collection: %d", rec.Code)
	}
	if len(names()) != 0 {
		t.Fatalf("nothing shows before the Collection is on = %v", names())
	}
	rec = switchTo("dm", "true")
	if list := decodeList(t, rec); rec.Code != http.StatusOK || len(list) != 1 || list[0]["switchedOn"] != true {
		t.Fatalf("switched on: %d %v", rec.Code, list)
	}
	if got := names(); len(got) != 2 {
		t.Fatalf("the Collection brings both spells in = %v", got)
	}
	ward := decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/library", "dm", ""))[0]
	if ward["direct"] != false || ward["via"].([]any)[0] != "Feywild Expansion" {
		t.Fatalf("an entry brought in by the Collection = %v", ward)
	}
	over := "/api/v1/campaigns/" + campaign + "/library/" + moon + "/override"
	if rec := call(h, http.MethodPut, over, "dm", `{"fields":[{"name":"Level","value":"2"}]}`); rec.Code != http.StatusOK || fieldsOf(t, decode(t, rec)["fields"])["Level"] != "2" {
		t.Fatalf("overriding a Collection's entry: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, "/api/v1/campaigns/"+campaign+"/library/"+sprite+"/pin", "dm", `{"revision":1}`); rec.Code != http.StatusOK {
		t.Fatalf("pinning a Collection's entry: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, "/api/v1/campaigns/"+campaign+"/library/"+moon, "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unlinking what the DM never linked: %d", rec.Code)
	}
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+sprite+`"}`)
	if rec := switchTo("dm", "false"); rec.Code != http.StatusOK || decodeList(t, rec)[0]["switchedOn"] != false {
		t.Fatalf("switched off: %d", rec.Code)
	}
	if got := names(); len(got) != 1 || got[0] != "Sprite Call" {
		t.Fatalf("off hides the Collection's entries except one linked directly = %v", got)
	}
	switchTo("dm", "true")
	if l := decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/library", "dm", ""))[0]; fieldsOf(t, l["fields"])["Level"] != "2" {
		t.Fatalf("back on, the override is kept = %v", l)
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/collections", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a player lists the Campaign's Collections: %d", rec.Code)
	}
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/collections", "dm", "")); len(list) != 1 || list[0]["switchedOn"] != true {
		t.Fatalf("the Campaign's Collections = %v", list)
	}
	if rec := call(h, http.MethodPut, "/api/v1/campaigns/"+campaign+"/collections/"+theirs, "dm", `{"on":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("switching on no such Collection: %d", rec.Code)
	}
}

// A Player proposes; the DM asks for changes, approves with an edit into their Library and the Campaign
// Collection, approves a change to one of their entries as its next Revision, and declines; the bell
// rings for every step.
func TestProposalsEveryOutcome(t *testing.T) {
	t.Parallel()
	bell := &notices{}
	h := libraryStackWith(t, bell)
	campaign, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + campaign + "/proposals"
	propose := func(body string) *httptest.ResponseRecorder { return call(h, http.MethodPost, base, "player", body) }
	rec := propose(`{"kind":"spell","name":"Frost Lance","fields":[{"name":"Damage","value":"4d10"}],"note":"For my wizard"}`)
	lance := decode(t, rec)
	id := lance["id"].(string)
	if rec.Code != http.StatusCreated || lance["status"] != "pending" || lance["authorName"] == "" || lance["note"] != "For my wizard" {
		t.Fatalf("propose: %d %v", rec.Code, lance)
	}
	if n := bell.last("dm"); !strings.HasSuffix(n.Title, " proposes Frost Lance") || n.Label != "Review" || n.Path != "/campaigns/"+campaign+"/proposals/"+id {
		t.Fatalf("the DM hears = %+v", n)
	}
	for body, want := range map[string]int{
		`{"kind":"vehicle","name":"Cart","fields":[]}`:                                       http.StatusBadRequest,
		`{"kind":"spell","name":"Odd","fields":[],"baseEntryId":"` + uuid.NewString() + `"}`: http.StatusNotFound,
	} {
		if rec := propose(body); rec.Code != want {
			t.Fatalf("%s: %d", body, rec.Code)
		}
	}
	if list := decodeList(t, call(h, http.MethodGet, base, "dm", "")); len(list) != 1 {
		t.Fatalf("the DM's list = %v", list)
	}
	one := base + "/" + id
	review := func(who, body string) *httptest.ResponseRecorder {
		return call(h, http.MethodPost, one+"/review", who, body)
	}
	if rec := review("player", `{"action":"approve"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player approves: %d", rec.Code)
	}
	if rec := review("dm", `{"action":"request_changes"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("asking for changes without saying what: %d", rec.Code)
	}
	rec = review("dm", `{"action":"request_changes","message":"Lower the damage"}`)
	if d := decode(t, rec); rec.Code != http.StatusOK || d["proposal"].(map[string]any)["status"] != "changes_requested" || d["proposal"].(map[string]any)["message"] != "Lower the damage" {
		t.Fatalf("changes asked: %d %v", rec.Code, d)
	}
	if n := bell.last("player"); n.Title != "Joris asks for changes to Frost Lance" || n.Body != "Lower the damage" || n.Label != "Open" {
		t.Fatalf("the player hears = %+v", n)
	}
	if rec := review("dm", `{"action":"approve"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reviewing a Proposal that waits on its author: %d", rec.Code)
	}
	resubmit := func(who string) *httptest.ResponseRecorder {
		return call(h, http.MethodPut, one, who, `{"name":"Frost Lance","fields":[{"name":"Damage","value":"3d10"}],"note":"Lowered"}`)
	}
	if rec := resubmit("dm"); rec.Code != http.StatusForbidden {
		t.Fatalf("a DM resubmits a player's Proposal: %d", rec.Code)
	}
	if rec := resubmit("player"); rec.Code != http.StatusOK || decode(t, rec)["proposal"].(map[string]any)["status"] != "pending" {
		t.Fatalf("resubmit: %d", rec.Code)
	}
	if n := bell.last("dm"); !strings.HasSuffix(n.Title, " changed Frost Lance") {
		t.Fatalf("the DM hears of the change = %+v", n)
	}
	if rec := resubmit("player"); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("resubmitting a pending Proposal: %d", rec.Code)
	}
	rec = review("dm", `{"action":"approve","message":"Welcome","fields":[{"name":"Damage","value":"3d8"}]}`)
	approved := decode(t, rec)
	p := approved["proposal"].(map[string]any)
	if rec.Code != http.StatusOK || p["status"] != "approved" || p["entryId"] == nil || fieldsOf(t, p["fields"])["Damage"] != "3d8" {
		t.Fatalf("approve: %d %v", rec.Code, approved)
	}
	var actions []string
	for _, st := range approved["steps"].([]any) {
		actions = append(actions, st.(map[string]any)["action"].(string))
	}
	if strings.Join(actions, ",") != "submitted,changes_requested,resubmitted,approved" {
		t.Fatalf("the history = %v", actions)
	}
	if n := bell.last("player"); n.Title != "Joris approved Frost Lance" {
		t.Fatalf("the player hears of the approval = %+v", n)
	}
	mine := decodeList(t, call(h, http.MethodGet, "/api/v1/library", "dm", ""))
	if len(mine) != 1 || mine[0]["name"] != "Frost Lance" || mine[0]["id"] != p["entryId"] {
		t.Fatalf("approval copies it into the DM's Library = %v", mine)
	}
	linked := decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/library", "dm", ""))
	if len(linked) != 1 || linked[0]["via"].([]any)[0] != "Campaign Collection" {
		t.Fatalf("and links it through the Campaign Collection = %v", linked)
	}

	hag := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"52"}]}`))["id"].(string)
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+hag+`"}`)
	change := decode(t, propose(`{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"40"}],"baseEntryId":"`+hag+`"}`))["id"].(string)
	d := decode(t, call(h, http.MethodGet, base+"/"+change, "dm", ""))
	if cur := d["current"].(map[string]any); fieldsOf(t, cur["fields"])["HP"] != "52" {
		t.Fatalf("the review shows the entry as the Campaign sees it = %v", d)
	}
	rec = call(h, http.MethodPost, base+"/"+change+"/review", "dm", `{"action":"approve","name":"Bog Hag Elder"}`)
	if p := decode(t, rec)["proposal"].(map[string]any); rec.Code != http.StatusOK || p["entryId"] != hag {
		t.Fatalf("a change to the DM's own entry: %d %v", rec.Code, p)
	}
	entry := decode(t, call(h, http.MethodGet, "/api/v1/library/"+hag, "dm", ""))["entry"].(map[string]any)
	if entry["revision"] != float64(2) || entry["name"] != "Bog Hag Elder" || fieldsOf(t, entry["fields"])["HP"] != "40" {
		t.Fatalf("becomes its next Revision = %v", entry)
	}

	no := decode(t, propose(`{"kind":"item","name":"Vorpal Spoon","fields":[]}`))["id"].(string)
	if rec := call(h, http.MethodPost, base+"/"+no+"/review", "dm", `{"action":"decline","message":"No"}`); rec.Code != http.StatusOK {
		t.Fatalf("decline: %d", rec.Code)
	}
	if n := bell.last("player"); n.Title != "Joris declined Vorpal Spoon" || n.Body != "No" {
		t.Fatalf("the player hears of the decline = %+v", n)
	}
	if rec := call(h, http.MethodPost, base+"/"+no+"/review", "dm", `{"action":"approve"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("approving a declined Proposal: %d", rec.Code)
	}
	again := decode(t, propose(`{"kind":"item","name":"Ladle","fields":[]}`))["id"].(string)
	call(h, http.MethodPost, base+"/"+again+"/review", "dm", `{"action":"approve"}`)
	var homes int
	for _, c := range decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/collections", "dm", "")) {
		if c["name"] == "Campaign Collection" && c["switchedOn"] == true {
			homes++
		}
	}
	if homes != 1 {
		t.Fatalf("one Campaign Collection, made once = %d", homes)
	}
	if list := decodeList(t, call(h, http.MethodGet, base, "player", "")); len(list) != 4 {
		t.Fatalf("the player sees their four Proposals = %v", list)
	}
	if rec := call(h, http.MethodGet, base+"/"+uuid.NewString(), "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no such Proposal: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, base+"/"+again+"/review", "dm", `{"action":"shrug"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown review: %d", rec.Code)
	}
}

// Refusals across the Library: blank names, players changing a Campaign's links, and Proposals read or
// sent by someone who may not.
func TestLibraryRefusals(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	entry := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+entry+`"}`)
	col := decode(t, call(h, http.MethodPost, "/api/v1/library/collections", "dm", `{"name":"Fey"}`))["id"].(string)
	base := "/api/v1/campaigns/" + campaign
	mine := decode(t, call(h, http.MethodPost, base+"/proposals", "dm", `{"kind":"npc","name":"Mine","fields":[]}`))["id"].(string)
	theirs := decode(t, call(h, http.MethodPost, base+"/proposals", "player", `{"kind":"npc","name":"Theirs","fields":[]}`))["id"].(string)
	call(h, http.MethodPost, base+"/proposals/"+theirs+"/review", "dm", `{"action":"request_changes","message":"More"}`)
	other := decode(t, call(h, http.MethodPost, base+"/proposals", "player", `{"kind":"npc","name":"Other","fields":[]}`))["id"].(string)
	for _, c := range []struct {
		name, method, path, who, body string
		want                          int
	}{
		{"a blank name on an entry", http.MethodPut, "/api/v1/library/" + entry, "dm", `{"name":" ","fields":[]}`, http.StatusUnprocessableEntity},
		{"a blank name on a Collection", http.MethodPut, "/api/v1/library/collections/" + col, "dm", `{"name":" ","entryIds":[]}`, http.StatusUnprocessableEntity},
		{"a player unlinks", http.MethodDelete, base + "/library/" + entry, "player", "", http.StatusForbidden},
		{"a player overrides", http.MethodPut, base + "/library/" + entry + "/override", "player", `{"fields":[]}`, http.StatusForbidden},
		{"a stranger proposes", http.MethodPost, base + "/proposals", "stranger", `{"kind":"npc","name":"X","fields":[]}`, http.StatusNotFound},
		{"a stranger lists Proposals", http.MethodGet, base + "/proposals", "stranger", "", http.StatusNotFound},
		{"a stranger reads a Proposal", http.MethodGet, base + "/proposals/" + theirs, "stranger", "", http.StatusNotFound},
		{"a player reads the DM's Proposal", http.MethodGet, base + "/proposals/" + mine, "player", "", http.StatusNotFound},
		{"a blank proposed name", http.MethodPost, base + "/proposals", "player", `{"kind":"npc","name":" ","fields":[]}`, http.StatusUnprocessableEntity},
		{"resending with a blank name", http.MethodPut, base + "/proposals/" + theirs, "player", `{"name":" ","fields":[]}`, http.StatusUnprocessableEntity},
		{"approving with a blank name", http.MethodPost, base + "/proposals/" + other + "/review", "dm", `{"action":"approve","name":" "}`, http.StatusUnprocessableEntity},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

// admins are the subjects with Admin powers.
type admins map[string]bool

func (a admins) IsAdmin(_ context.Context, subject string) bool { return a[subject] }

// A DM asks to share an entry; an Admin approves it only with the IP check, which puts a read-only copy
// in the Shared Library that another DM links; a declined request keeps its IP check too.
func TestSharedLibraryWithAdminReview(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	hag := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"52"}]}`))["id"].(string)
	share := func(who, entry string) *httptest.ResponseRecorder {
		return call(h, http.MethodPost, "/api/v1/shared-library/submissions", who, `{"entryId":"`+entry+`","note":"All my own words"}`)
	}
	if rec := share("player", hag); rec.Code != http.StatusNotFound {
		t.Fatalf("sharing another account's entry: %d", rec.Code)
	}
	rec := share("dm", hag)
	sub := decode(t, rec)
	if rec.Code != http.StatusCreated || sub["status"] != "pending" || sub["revision"] != float64(1) || sub["ipClear"] != nil {
		t.Fatalf("share: %d %v", rec.Code, sub)
	}
	if rec := share("dm", hag); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sharing what already waits: %d", rec.Code)
	}
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/shared-library/submissions", "dm", "")); len(list) != 1 {
		t.Fatalf("my requests = %v", list)
	}
	review := "/api/v1/admin/shared-library/" + sub["id"].(string) + "/review"
	if rec := call(h, http.MethodGet, "/api/v1/admin/shared-library", "dm", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a DM lists the requests: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, review, "dm", `{"decision":"approve","ipClear":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a DM approves their own request: %d", rec.Code)
	}
	if list := decodeList(t, call(h, http.MethodGet, "/api/v1/admin/shared-library", "admin", "")); len(list) != 1 || list[0]["status"] != "pending" {
		t.Fatalf("the Admin's queue = %v", list)
	}
	if rec := call(h, http.MethodPost, review, "admin", `{"decision":"approve","ipClear":false}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("approving without the IP check: %d", rec.Code)
	}
	call(h, http.MethodPut, "/api/v1/library/"+hag, "dm", `{"name":"Bog Hag","fields":[{"name":"HP","value":"99"}]}`)
	rec = call(h, http.MethodPost, review, "admin", `{"decision":"approve","ipClear":true,"ipNote":"No non-SRD text","message":"Thanks"}`)
	done := decode(t, rec)
	if rec.Code != http.StatusOK || done["status"] != "approved" || done["ipClear"] != true || done["ipNote"] != "No non-SRD text" || done["sharedEntryId"] == nil || done["decidedAt"] == nil {
		t.Fatalf("approve: %d %v", rec.Code, done)
	}
	if rec := call(h, http.MethodPost, review, "admin", `{"decision":"decline","ipClear":false}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("reviewing twice: %d", rec.Code)
	}
	shared := decodeList(t, call(h, http.MethodGet, "/api/v1/shared-library?kind=creature", "player", ""))
	if len(shared) != 1 || shared[0]["shared"] != true || fieldsOf(t, shared[0]["fields"])["HP"] != "52" || shared[0]["id"] == hag {
		t.Fatalf("the Shared Library holds a copy of the submitted Revision = %v", shared)
	}
	copied := shared[0]["id"].(string)

	own := decode(t, call(h, http.MethodPost, "/api/v1/campaigns", "player", `{"name":"Elsewhere","displayName":"Tamsin"}`))["id"].(string)
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+own+"/library", "player", `{"entryId":"`+copied+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("another DM links the shared copy: %d %s", rec.Code, rec.Body.String())
	}
	if d := decode(t, call(h, http.MethodGet, "/api/v1/library/"+copied, "player", "")); len(d["uses"].([]any)) != 0 || d["entry"].(map[string]any)["shared"] != true {
		t.Fatalf("a shared copy reads without where others use it = %v", d)
	}
	for _, who := range []string{"player", "dm", "admin"} {
		if rec := call(h, http.MethodPut, "/api/v1/library/"+copied, who, `{"name":"Mine now","fields":[]}`); rec.Code != http.StatusNotFound {
			t.Fatalf("%s edits the shared copy: %d", who, rec.Code)
		}
	}

	odo := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	second := decode(t, share("dm", odo))["id"].(string)
	rec = call(h, http.MethodPost, "/api/v1/admin/shared-library/"+second+"/review", "admin", `{"decision":"decline","ipClear":false,"ipNote":"Quotes a published book","message":"Rewrite it in your own words"}`)
	if no := decode(t, rec); rec.Code != http.StatusOK || no["status"] != "declined" || no["ipClear"] != false || no["sharedEntryId"] != nil {
		t.Fatalf("decline: %d %v", rec.Code, no)
	}
	if rec := call(h, http.MethodPost, "/api/v1/admin/shared-library/"+uuid.NewString()+"/review", "admin", `{"decision":"decline","ipClear":false}`); rec.Code != http.StatusNotFound {
		t.Fatalf("no such request: %d", rec.Code)
	}
}

// Homebrew exported in Grimoire's own schema imports into another Library unchanged, and an import
// reports what it could not take as Manual.
func TestHomebrewExportAndImport(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	hag := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"52"},{"name":"Lair","value":"Fen"}]}`))["id"].(string)
	lance := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Frost Lance","fields":[{"name":"Damage","value":"3d8"}]}`))["id"].(string)
	call(h, http.MethodPut, "/api/v1/builders/spells/"+lance, "dm", lanternDesign)
	col := decode(t, call(h, http.MethodPost, "/api/v1/library/collections", "dm", `{"name":"Fen","description":"Bog things"}`))["id"].(string)
	call(h, http.MethodPut, "/api/v1/library/collections/"+col, "dm", `{"name":"Fen","description":"Bog things","entryIds":["`+hag+`"]}`)

	rec := call(h, http.MethodGet, "/api/v1/library/export", "dm", "")
	doc := decode(t, rec)
	if rec.Code != http.StatusOK || doc["format"] != "grimoire-library" || doc["version"] != float64(1) || len(doc["entries"].([]any)) != 2 || len(doc["collections"].([]any)) != 1 {
		t.Fatalf("export: %d %v", rec.Code, doc)
	}
	if one := decode(t, call(h, http.MethodGet, "/api/v1/library/export?collectionId="+col, "dm", "")); len(one["entries"].([]any)) != 1 || len(one["collections"].([]any)) != 1 {
		t.Fatalf("a Collection's export = %v", one)
	}
	if one := decode(t, call(h, http.MethodGet, "/api/v1/library/export?entryId="+hag, "dm", "")); len(one["entries"].([]any)) != 1 || len(one["collections"].([]any)) != 0 {
		t.Fatalf("an entry's export = %v", one)
	}
	for _, q := range []string{"?entryId=" + hag, "?collectionId=" + col} {
		if rec := call(h, http.MethodGet, "/api/v1/library/export"+q, "player", ""); rec.Code != http.StatusNotFound {
			t.Fatalf("exporting another account's %s: %d", q, rec.Code)
		}
	}

	rec = call(h, http.MethodPost, "/api/v1/library/import", "player", mustJSON(t, doc))
	report := decode(t, rec)
	if rec.Code != http.StatusOK || len(report["entries"].([]any)) != 2 || len(report["collections"].([]any)) != 1 || len(report["manual"].([]any)) != 0 {
		t.Fatalf("import: %d %v", rec.Code, report)
	}
	again := decode(t, call(h, http.MethodGet, "/api/v1/library/export", "player", ""))
	same := func(d map[string]any) string {
		var out []string
		for _, e := range d["entries"].([]any) {
			m := e.(map[string]any)
			out = append(out, m["kind"].(string)+"/"+m["name"].(string)+"/"+mustJSON(t, m["fields"])+"/"+mustJSON(t, m["parts"]))
		}
		for _, c := range d["collections"].([]any) {
			m := c.(map[string]any)
			out = append(out, m["name"].(string)+"/"+m["description"].(string)+"/"+strings.Repeat("e", len(m["entries"].([]any))))
		}
		return strings.Join(out, "\n")
	}
	if same(again) != same(doc) {
		t.Fatalf("the round trip changed something:\n%s\nwas\n%s", same(again), same(doc))
	}

	odd := `{"format":"grimoire-library","version":1,"entries":[
		{"key":"a","kind":"creature","name":"Ogre","fields":{"HP":59,"AC":"11"},"parts":[{"type":"effect"}]},
		{"key":"b","kind":"vehicle","name":"Cart"},
		{"key":"c","kind":"spell","name":"Odd","parts":[{"type":"spell","design":{"targeting":{"shape":"blob","sizeFt":5,"rangeFt":0}}}]},
		{"key":"d","kind":"subclass","name":"Tinker","parts":[{"type":"subclass","design":{"class":"artificer","features":[]}}]},
		{"key":"e","kind":"item","name":"Spoon","parts":[{"type":"item","design":{"kind":"spoon"}}]}],
		"collections":[{"name":"Mixed","description":"","entries":["a","b"]}]}`
	report = decode(t, call(h, http.MethodPost, "/api/v1/library/import", "player", odd))
	if len(report["entries"].([]any)) != 4 || len(report["manual"].([]any)) != 7 {
		t.Fatalf("an import with odd parts = %v", report)
	}
	if rec := call(h, http.MethodPost, "/api/v1/library/import", "player", `{"format":"other","version":1,"entries":[]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("another format: %d", rec.Code)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

const lanternDesign = `{"targeting":{"shape":"emanation","sizeFt":10,"rangeFt":0},"save":"wisdom","concentration":true,
"duration":{"unit":"minutes","amount":1},"ritual":true,"castingTime":{"kind":"action"},
"components":{"verbal":true,"somatic":false,"material":{"text":"a lantern of bog glass","costGp":25,"consumed":true,"item":"lantern"}},
"parts":[{"type":"light","brightFt":20,"dimFt":20},{"type":"reveal","qualities":["invisible"]},
{"type":"condition","condition":"charmed","onlyTypes":["undead","fey"]},{"type":"damage","when":"start_of_turn","dice":"1d6","damageType":"radiant"}]}`

// An author builds a spell in the Effect builder: a fresh spell opens with a starting design, a preview
// draws the area and writes the rules text, a design the rules cannot run is refused with the reason,
// and saving keeps the design as the spell's next Revision.
func TestTheEffectBuilder(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	spell := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Marsh Lantern","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/spells/" + spell
	rec := call(h, http.MethodGet, path, "dm", "")
	fresh := decode(t, rec)
	if rec.Code != http.StatusOK || fresh["design"].(map[string]any)["targeting"].(map[string]any)["shape"] != "sphere" || !strings.HasPrefix(fresh["effect"].(string), "hb-") {
		t.Fatalf("a fresh spell: %d %v", rec.Code, fresh)
	}
	rec = call(h, http.MethodPost, "/api/v1/builders/spells/preview", "dm", `{"name":"Marsh Lantern","design":`+lanternDesign+`}`)
	preview := decode(t, rec)
	text := strings.Join(toStrings(preview["text"].([]any)), "\n")
	if rec.Code != http.StatusOK || len(preview["hexes"].([]any)) != 18 || !strings.Contains(text, "Casting Time: 1 action or Ritual.") ||
		!strings.Contains(text, "has the Charmed condition, if it is Undead or Fey.") || !strings.Contains(text, "starts its turn in the area takes 1d6 radiant damage") {
		t.Fatalf("preview: %d %v", rec.Code, preview)
	}
	bad := strings.Replace(lanternDesign, `"save":"wisdom",`, "", 1)
	if rec := call(h, http.MethodPost, "/api/v1/builders/spells/preview", "dm", `{"name":"X","design":`+bad+`}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "needs the save") {
		t.Fatalf("a condition without a save: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, path, "player", lanternDesign); rec.Code != http.StatusNotFound {
		t.Fatalf("saving another account's spell: %d", rec.Code)
	}
	for _, p := range []string{"/api/v1/builders/spells/" + npc} {
		if rec := call(h, http.MethodGet, p, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an NPC in the spell builder: %d", rec.Code)
		}
		if rec := call(h, http.MethodPut, p, "dm", lanternDesign); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("saving a design on an NPC: %d", rec.Code)
		}
	}
	rec = call(h, http.MethodPut, path, "dm", lanternDesign)
	saved := decode(t, rec)
	if rec.Code != http.StatusOK || saved["entry"].(map[string]any)["revision"] != float64(2) || len(saved["design"].(map[string]any)["parts"].([]any)) != 4 {
		t.Fatalf("save: %d %v", rec.Code, saved)
	}
	if again := decode(t, call(h, http.MethodGet, path, "dm", "")); again["design"].(map[string]any)["save"] != "wisdom" {
		t.Fatalf("the design is kept = %v", again)
	}
	call(h, http.MethodPut, "/api/v1/library/"+spell, "dm", `{"name":"Marsh Lantern","fields":[{"name":"School","value":"Evocation"}]}`)
	if kept := decode(t, call(h, http.MethodGet, path, "dm", "")); kept["design"].(map[string]any)["save"] != "wisdom" {
		t.Fatalf("editing the fields keeps the design = %v", kept)
	}
}

func toStrings(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, v.(string))
	}
	return out
}

// mute is a bell that never rings.
type mute struct{}

func (mute) Notify(context.Context, string, libraryapp.Notice) error {
	return errors.New("bell broken")
}

func TestABrokenBellNeverUndoesAProposal(t *testing.T) {
	t.Parallel()
	h := libraryStackWith(t, mute{})
	campaign, _ := campaignWithPlayer(t, h)
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/proposals", "player", `{"kind":"npc","name":"Odo","fields":[]}`); rec.Code != http.StatusCreated {
		t.Fatalf("a Proposal with a broken bell: %d %s", rec.Code, rec.Body.String())
	}
}

// failingLibrary fails as a broken database would.
type failingLibrary struct{ httpapi.LibraryService }

func (failingLibrary) Entries(context.Context, caller.Caller, string) ([]domain.Entry, error) {
	return nil, errors.New("library down")
}

func TestABrokenLibraryAnswersUnavailable(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, nil, httpapi.LibraryService(failingLibrary{}))
	if rec := call(h, http.MethodGet, "/api/v1/library", "dm", ""); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "down") {
		t.Fatalf("a broken Library: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/library", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", rec.Code)
	}
}
