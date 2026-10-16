//go:build integration

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestRollsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/rolls"
	body := `{"purpose":"Attack","notation":"2d20kh1+1d4","labels":[{"group":1,"label":"Bless"}],"modifiers":[{"label":"Strength","value":3}]}`
	rec := call(h, http.MethodPost, base, "player", body)
	roll := decode(t, rec)
	rollID, _ := roll["id"].(string)
	groups, _ := roll["groups"].([]any)
	if rec.Code != http.StatusCreated || roll["mine"] != true || roll["canRoll"] != true || len(groups) != 2 ||
		!strings.Contains(rec.Body.String(), `"keep":"highest","keepCount":1`) || !strings.Contains(rec.Body.String(), `"label":"Bless"`) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	one := base + "/" + rollID
	if rec := call(h, http.MethodPost, one+"/dice/0", "player", `{"mode":"manual","value":18}`); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), `"value":18,"mode":"manual"`) {
		t.Fatalf("manual: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, one+"/dice/1", "player", `{"mode":"manual","value":21}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("face 21: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, one+"/dice/1", "player", `{"mode":"auto"}`); rec.Code != 200 {
		t.Fatalf("auto: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, one+"/rest", "player", "")
	done := decode(t, rec)
	if rec.Code != 200 || done["status"] != "resolved" || done["total"] == nil || done["resolvedAt"] == nil {
		t.Fatalf("rest: %d %s", rec.Code, rec.Body.String())
	}
	checkRollReads(t, h, id, rollID)
}

func checkRollReads(t *testing.T, h http.Handler, id, rollID string) {
	t.Helper()
	base := "/api/v1/campaigns/" + id + "/rolls"
	one := base + "/" + rollID
	if rec := call(h, http.MethodGet, one, "dm", ""); rec.Code != 200 || decode(t, rec)["canRoll"] != true {
		t.Fatalf("dm get: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base+"?limit=5", "player", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), rollID) {
		t.Fatalf("list: %d", rec.Code)
	}
	rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/log", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"kind":"die_rolled"`) || !strings.Contains(rec.Body.String(), `"seed":"`) {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/log", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player log: %d", rec.Code)
	}
	home := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+id, "dm", ""))
	members, _ := home["members"].([]any)
	playerRow, _ := members[1].(map[string]any)
	playerID, _ := playerRow["id"].(string)
	ask := `{"purpose":"Stealth","notation":"1d20","rollerId":"` + playerID + `"}`
	if rec := call(h, http.MethodPost, base, "dm", ask); rec.Code != http.StatusCreated || decode(t, rec)["mine"] != false {
		t.Fatalf("dm asks player: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, base, "player", `{"purpose":"x","notation":"1d7"}`); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "there is no d7") {
		t.Fatalf("bad notation: %d %s", rec.Code, rec.Body.String())
	}
}
