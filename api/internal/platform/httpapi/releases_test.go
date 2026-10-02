package httpapi_test

import (
	"context"
	"net/http"
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
func TestReleaseNotes(t *testing.T) {
	t.Parallel()
	st := newStack(t, func(*identityapp.Service) {})
	h := st.public
	aria, bram := setUp(t, st.trusted, h, "aria"), setUp(t, st.trusted, h, "bram")
	if rec := send(h, http.MethodPost, "/api/v1/admin/release-notes", aria, "", `{"version":"1.1.0"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a non-Admin drafts: %d", rec.Code)
	}
	rec := send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes", "", "root", `{"version":"v1.1.0"}`)
	draft := decode(t, rec)
	id, _ := draft["id"].(string)
	if rec.Code != http.StatusCreated || draft["status"] != "draft" || draft["version"] != "1.1.0" || draft["title"] != "What is new in Grimoire 1.1.0" ||
		draft["body"] != "- Social: Friends with requests\n- Release Notes" {
		t.Fatalf("draft: %d %v", rec.Code, draft)
	}
	for body, want := range map[string]int{`{"version":"1.1.0"}`: http.StatusConflict, `{"version":"1.2.0-rc.1"}`: http.StatusUnprocessableEntity} {
		if rec := send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes", "", "root", body); rec.Code != want {
			t.Errorf("draft %s = %d", body, rec.Code)
		}
	}
	rec = send(st.trusted, http.MethodPut, "/api/v1/admin/release-notes/"+id, "", "root", `{"title":" Friends arrive ","body":"- You can now add Friends."}`)
	if rec.Code != http.StatusOK || decode(t, rec)["title"] != "Friends arrive" {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	later := st.now.Now().Add(time.Hour).Format(time.RFC3339)
	rec = send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes/"+id+"/publish", "", "root", `{"at":"`+later+`"}`)
	if rec.Code != http.StatusOK || decode(t, rec)["status"] != "scheduled" {
		t.Fatalf("schedule: %d %s", rec.Code, rec.Body.String())
	}
	if note := unseen(t, h, aria); note != nil {
		t.Fatalf("a scheduled note shows = %v", note)
	}
	if rec := send(h, http.MethodPost, "/api/v1/release-notes/"+id+"/seen", aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("seeing a note not live yet: %d", rec.Code)
	}
	st.now.pass(2 * time.Hour)
	if err := st.social.AnnounceReleases(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := bell(t, h, bram); len(got) != 1 || got[0]["kind"] != "release_note" || got[0]["title"] != "Friends arrive" || got[0]["actionPath"] != "/" {
		t.Fatalf("bram's bell = %v", got)
	}
	if note := unseen(t, h, aria); note == nil || note["id"] != id || note["status"] != "published" {
		t.Fatalf("aria's Dashboard = %v", note)
	}
	if rec := send(h, http.MethodPost, "/api/v1/release-notes/"+id+"/seen", aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("seen: %d", rec.Code)
	}
	if note := unseen(t, h, aria); note != nil {
		t.Fatalf("a seen note shows again = %v", note)
	}
	if note := unseen(t, h, bram); note == nil {
		t.Fatal("bram has not seen it yet")
	}
	if rec := send(st.trusted, http.MethodPut, "/api/v1/admin/release-notes/"+id, "", "root", `{"title":"Late","body":""}`); rec.Code != http.StatusConflict {
		t.Fatalf("editing an announced note: %d", rec.Code)
	}
	rec = send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes", "", "root", `{"version":"1.2.0"}`)
	next, _ := decode(t, rec)["id"].(string)
	if rec := send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes/"+next+"/publish", "", "root", `{}`); rec.Code != http.StatusOK || decode(t, rec)["status"] != "published" {
		t.Fatalf("publish now: %d", rec.Code)
	}
	if list := decode(t, send(st.trusted, http.MethodGet, "/api/v1/admin/release-notes", "", "root", ""))["items"].([]any); len(list) != 2 {
		t.Fatalf("list = %v", list)
	}
	if rec := send(st.trusted, http.MethodPost, "/api/v1/admin/release-notes/0190c7a8-0000-7000-8000-0000000000c1/publish", "", "root", `{}`); rec.Code != http.StatusNotFound {
		t.Fatalf("a missing note: %d", rec.Code)
	}
}
