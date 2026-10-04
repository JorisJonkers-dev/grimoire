package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// Every Revision of a Library entry says how it was made, and an earlier one is brought back as the
// entry's next Revision: its name, its fields and, for an entry made in a builder, its design.
func TestLibraryRevisionsSayHowTheyWereMadeAndCanBeRestored(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	hag := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog Hag","fields":[{"name":"HP","value":"52"}]}`))["id"].(string)
	call(h, http.MethodPut, "/api/v1/library/"+hag, "dm", `{"name":"Bog Hag Matriarch","fields":[{"name":"HP","value":"80"},{"name":"AC","value":"18"}]}`)
	revisions := func(id string) []map[string]any {
		t.Helper()
		out := []map[string]any{}
		for _, r := range decode(t, call(h, http.MethodGet, "/api/v1/library/"+id, "dm", ""))["revisions"].([]any) {
			out = append(out, r.(map[string]any))
		}
		return out
	}
	if revs := revisions(hag); len(revs) != 2 || revs[0]["no"] != float64(2) || revs[0]["origin"] != "ui" || revs[1]["origin"] != "ui" || revs[0]["client"] != nil {
		t.Fatalf("two Revisions made by hand = %v", revs)
	}

	rec := call(h, http.MethodPost, "/api/v1/library/"+hag+"/revisions/1/restore", "dm", "")
	restored := decode(t, rec)
	entry, _ := restored["entry"].(map[string]any)
	if rec.Code != http.StatusOK || entry["name"] != "Bog Hag" || entry["revision"] != float64(3) || len(entry["fields"].([]any)) != 1 || fieldsOf(t, entry["fields"])["HP"] != "52" {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	if revs := revisions(hag); len(revs) != 3 || revs[0]["no"] != float64(3) || revs[0]["name"] != "Bog Hag" || revs[1]["name"] != "Bog Hag Matriarch" || revs[2]["name"] != "Bog Hag" {
		t.Fatalf("restoring adds a Revision and loses none = %v", revs)
	}

	// A Roll Table made in the builder: bringing an earlier Revision back brings its design back.
	table := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"table","name":"Fumbles","fields":[]}`))["id"].(string)
	build := "/api/v1/builders/roll-tables/" + table
	if rec := call(h, http.MethodPut, build, "dm", fumbleDesign); rec.Code != http.StatusOK {
		t.Fatalf("save the design: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, build, "dm", `{"dice":"1d4","results":[{"from":1,"to":4,"text":"Nothing much."}]}`); rec.Code != http.StatusOK {
		t.Fatalf("save another design: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, "/api/v1/library/"+table+"/revisions/2/restore", "dm", ""); rec.Code != http.StatusOK {
		t.Fatalf("restore the first design: %d %s", rec.Code, rec.Body.String())
	}
	back := decode(t, call(h, http.MethodGet, build, "dm", ""))
	if design, _ := back["design"].(map[string]any); design["dice"] != "1d6" || len(design["results"].([]any)) != 3 {
		t.Fatalf("the design after restoring = %v", back["design"])
	}
	if revs := revisions(table); len(revs) != 4 {
		t.Fatalf("the table's Revisions = %v", revs)
	}
	// The Revision a restore makes holds the design too: it can itself be brought back.
	call(h, http.MethodPut, build, "dm", `{"dice":"1d4","results":[{"from":1,"to":4,"text":"Nothing much."}]}`)
	if rec := call(h, http.MethodPost, "/api/v1/library/"+table+"/revisions/4/restore", "dm", ""); rec.Code != http.StatusOK {
		t.Fatalf("restore the restored: %d %s", rec.Code, rec.Body.String())
	}
	if design, _ := decode(t, call(h, http.MethodGet, build, "dm", ""))["design"].(map[string]any); design["dice"] != "1d6" {
		t.Fatalf("the design after restoring a restored Revision = %v", design)
	}

	path := "/api/v1/library/" + hag + "/revisions/1/restore"
	for name, c := range map[string]struct {
		who, path string
		want      int
	}{
		"signed out":             {"", path, http.StatusUnauthorized},
		"another's entry":        {"player", path, http.StatusNotFound},
		"no such Revision":       {"dm", "/api/v1/library/" + hag + "/revisions/9/restore", http.StatusNotFound},
		"another entry's number": {"dm", "/api/v1/library/" + hag + "/revisions/4/restore", http.StatusNotFound},
		"no such entry":          {"dm", "/api/v1/library/0190c7a8-0000-7000-8000-0000000000ff/revisions/1/restore", http.StatusNotFound},
		"a Revision numbered 0":  {"dm", "/api/v1/library/" + hag + "/revisions/0/restore", http.StatusBadRequest},
	} {
		if rec := call(h, http.MethodPost, c.path, c.who, ""); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if len(revisions(hag)) != 3 {
		t.Fatalf("a refused restore added a Revision")
	}
	// An Access Token restores with build, and not with less.
	if rec := withScopes(h, http.MethodPost, path, "dm", "read play", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot build restores: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, path, "dm", "build", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"revision":4`) {
		t.Errorf("a token that can build: %d %s", rec.Code, rec.Body.String())
	}
}
