//go:build integration

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestNPCRevisionsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/npcs"
	rec := call(h, http.MethodPost, base, "dm", `{"name":"Morvain","title":"Count","dmNotes":"Old","disposition":"hostile"}`)
	npcID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || npcID == "" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	one := base + "/" + npcID
	if rec := call(h, http.MethodGet, base, "dm", ""); !strings.Contains(rec.Body.String(), `"title":"Count"`) {
		t.Fatalf("list: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodPut, one, "dm", `{"name":"Morvain","dmNotes":"New","disposition":"neutral"}`); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, one+"/revisions/diff?from=1&to=2", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"field":"dmNotes","before":"Old","after":"New"`) {
		t.Fatalf("diff: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, one+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["dmNotes"] != "Old" {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, one+"/revisions", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"restoredFrom":1`) || !strings.Contains(rec.Body.String(), `"origin":"ui"`) {
		t.Fatalf("revisions: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeletedNPCsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/npcs"
	npcID, _ := decode(t, call(h, http.MethodPost, base, "dm", `{"name":"Morvain","disposition":"hostile"}`))["id"].(string)
	one := base + "/" + npcID
	if rec := call(h, http.MethodGet, one, "dm", ""); rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec := call(h, http.MethodGet, base+"/deleted", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Morvain") {
		t.Fatalf("deleted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "dm", ""); rec.Code != 200 || rec.Body.String() != "[]" {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player: %d", rec.Code)
	}
}
