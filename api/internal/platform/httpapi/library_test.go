package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	repo := campaignpg.New(store.Pool())
	lib := &libraryapp.Service{Repo: librarypg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now}
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
