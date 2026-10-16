//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
)

func unseen(t *testing.T, h http.Handler, session string) map[string]any {
	t.Helper()
	rec := send(h, http.MethodGet, "/api/v1/release-notes/unseen", session, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("unseen: %d %s", rec.Code, rec.Body.String())
	}
	note, _ := decode(t, rec)["note"].(map[string]any)
	return note
}

// An Admin drafts the one Release Note of a full release from its changelog, edits it and schedules
// it; once live it rings every bell and shows on each Dashboard until seen.
// newsDesk is Aria and Bram, and a stack where root drafts Release Notes.
type newsDesk struct {
	st         stack
	aria, bram string
}

func newNewsDesk(t *testing.T) *newsDesk {
	t.Helper()
	st := newStack(t, func(*identityapp.Service) {})
	return &newsDesk{st: st, aria: setUp(t, st.trusted, st.public, "aria"), bram: setUp(t, st.trusted, st.public, "bram")}
}

func (n *newsDesk) root(method, path, body string) *httptest.ResponseRecorder {
	return send(n.st.trusted, method, "/api/v1/admin/release-notes"+path, "", "root", body)
}

// draft drafts the note for 1.1.0 and returns its id.
func (n *newsDesk) draft(t *testing.T) string {
	t.Helper()
	rec := n.root(http.MethodPost, "", `{"version":"v1.1.0"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("draft: %d", rec.Code)
	}
	id, _ := decode(t, rec)["id"].(string)
	return id
}

// live drafts, edits and schedules the 1.1.0 note an hour on, and announces it two hours on.
func (n *newsDesk) live(t *testing.T) string {
	t.Helper()
	id := n.draft(t)
	n.root(http.MethodPut, "/"+id, `{"title":" Friends arrive ","body":"- You can now add Friends."}`)
	n.root(http.MethodPost, "/"+id+"/publish", `{"at":"`+n.st.now.Now().Add(time.Hour).Format(time.RFC3339)+`"}`)
	n.st.now.pass(2 * time.Hour)
	if err := n.st.social.AnnounceReleases(context.Background()); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestOnlyAnAdminDraftsReleaseNotes(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	if rec := send(n.st.public, http.MethodPost, "/api/v1/admin/release-notes", n.aria, "", `{"version":"1.1.0"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}

func TestADraftIsWrittenFromTheChangelog(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	rec := n.root(http.MethodPost, "", `{"version":"v1.1.0"}`)
	if draft := decode(t, rec); rec.Code != http.StatusCreated || draft["status"] != "draft" || draft["version"] != "1.1.0" || draft["title"] != "What is new in Grimoire 1.1.0" ||
		draft["body"] != "- Social: Friends with requests\n- Release Notes" {
		t.Fatalf("draft: %d %v", rec.Code, draft)
	}
}

func TestAVersionIsDraftedOnceAndNeverAsAPreRelease(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	n.draft(t)
	for body, want := range map[string]int{`{"version":"1.1.0"}`: http.StatusConflict, `{"version":"1.2.0-rc.1"}`: http.StatusUnprocessableEntity} {
		t.Run(body, func(t *testing.T) {
			if rec := n.root(http.MethodPost, "", body); rec.Code != want {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestAnEditedNoteIsTrimmed(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	if rec := n.root(http.MethodPut, "/"+n.draft(t), `{"title":" Friends arrive ","body":"- You can now add Friends."}`); rec.Code != http.StatusOK || decode(t, rec)["title"] != "Friends arrive" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestANoteScheduledForLaterDoesNotShowYet(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	id := n.draft(t)
	if rec := n.root(http.MethodPost, "/"+id+"/publish", `{"at":"`+n.st.now.Now().Add(time.Hour).Format(time.RFC3339)+`"}`); rec.Code != http.StatusOK || decode(t, rec)["status"] != "scheduled" {
		t.Fatalf("schedule: %d", rec.Code)
	}
	if note := unseen(t, n.st.public, n.aria); note != nil {
		t.Fatalf("shows = %v", note)
	}
	if rec := send(n.st.public, http.MethodPost, "/api/v1/release-notes/"+id+"/seen", n.aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("seeing it: %d", rec.Code)
	}
}

func TestALiveNoteRingsEveryonesBell(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	n.live(t)
	if got, _ := bell(t, n.st.public, n.bram); len(got) != 1 || got[0]["kind"] != "release_note" || got[0]["title"] != "Friends arrive" || got[0]["actionPath"] != "/" {
		t.Fatalf("bram's bell = %v", got)
	}
}

func TestALiveNoteShowsUntilEachReaderHasSeenIt(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	id := n.live(t)
	if note := unseen(t, n.st.public, n.aria); note == nil || note["id"] != id || note["status"] != "published" {
		t.Fatalf("aria's Dashboard = %v", note)
	}
	if rec := send(n.st.public, http.MethodPost, "/api/v1/release-notes/"+id+"/seen", n.aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("seen: %d", rec.Code)
	}
	if note := unseen(t, n.st.public, n.aria); note != nil {
		t.Fatalf("shows again = %v", note)
	}
	if note := unseen(t, n.st.public, n.bram); note == nil {
		t.Fatal("gone for bram too")
	}
}

func TestAnAnnouncedNoteIsNotEdited(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	if rec := n.root(http.MethodPut, "/"+n.live(t), `{"title":"Late","body":""}`); rec.Code != http.StatusConflict {
		t.Fatalf("%d", rec.Code)
	}
}

func TestANoteWithoutATimeIsPublishedAtOnce(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	n.draft(t)
	next, _ := decode(t, n.root(http.MethodPost, "", `{"version":"1.2.0"}`))["id"].(string)
	if rec := n.root(http.MethodPost, "/"+next+"/publish", `{}`); rec.Code != http.StatusOK || decode(t, rec)["status"] != "published" {
		t.Fatalf("publish: %d", rec.Code)
	}
	if list := decode(t, n.root(http.MethodGet, "", ""))["items"].([]any); len(list) != 2 {
		t.Fatalf("list = %v", list)
	}
}

func TestPublishingAMissingNoteIsNotFound(t *testing.T) {
	t.Parallel()
	n := newNewsDesk(t)
	if rec := n.root(http.MethodPost, "/0190c7a8-0000-7000-8000-0000000000c1/publish", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}
