//go:build integration

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestCharacterLifecycleOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	rec := call(h, http.MethodPost, base+"/preview", "player", build)
	if rec.Code != 200 || decode(t, rec)["armorClass"] != float64(19) {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base, "player", build)
	sheet := decode(t, rec)
	chID, _ := sheet["id"].(string)
	if rec.Code != http.StatusCreated || sheet["hpMax"] != float64(12) || chID == "" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	bonus, _ := sheet["bonus"].(map[string]any)
	if len(bonus) != 3 {
		t.Fatalf("bonus = %v", bonus)
	}
	rec = call(h, http.MethodGet, base, "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ownerName":"Tamsin"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, base+"/"+chID, "dm", "")
	if rec.Code != 200 || decode(t, rec)["mine"] != false {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, base+"/"+chID, "player", `{"name":"Kara Two","hpCurrent":3,"armor":"","shield":false,"weapons":[]}`)
	edited := decode(t, rec)
	if rec.Code != 200 || edited["hpCurrent"] != float64(3) || edited["armorClass"] != float64(12) {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	bad := strings.Replace(build, `"skills":["perception","survival"]`, `"skills":["athletics","survival"]`, 1)
	rec = call(h, http.MethodPost, base, "player", bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "already grants athletics") {
		t.Fatalf("rule error: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, base+"/"+chID, "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}
