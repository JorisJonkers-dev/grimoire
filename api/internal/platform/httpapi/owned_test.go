package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// secondCampaign is another Campaign the player has joined, and its id.
func secondCampaign(t *testing.T, h http.Handler, name string) string {
	t.Helper()
	rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm2", `{"name":"`+name+`","displayName":"Ines"}`)
	id, _ := decode(t, rec)["id"].(string)
	token, _ := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/invites", "dm2", ""))["token"].(string)
	if rec := call(h, http.MethodPost, "/api/v1/invites/accept", "player", `{"token":"`+token+`","displayName":"Tamsin"}`); rec.Code != http.StatusOK {
		t.Fatalf("join %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return id
}

// A Character belongs to its player's Account and plays in a second Campaign with progress of its own;
// its name flows to every Campaign, and nobody else can see, change or bring it anywhere.
func TestCharactersOwnedByAccounts(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	first, _ := campaignWithPlayer(t, h)
	rec := call(h, http.MethodPost, "/api/v1/campaigns/"+first+"/characters", "player", build)
	sheet := decode(t, rec)
	firstSheet, _ := sheet["id"].(string)
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+first+"/characters/"+firstSheet, "player", `{"hpCurrent":5}`); rec.Code != http.StatusOK {
		t.Fatalf("hurt: %d %s", rec.Code, rec.Body.String())
	}
	list := decode(t, call(h, http.MethodGet, "/api/v1/characters", "player", ""))
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("mine = %v", list)
	}
	owned, _ := items[0].(map[string]any)
	ownedID, _ := owned["id"].(string)
	if owned["name"] != "Kara" || owned["class"] != "fighter" || len(owned["campaigns"].([]any)) != 1 {
		t.Fatalf("owned = %v", owned)
	}

	second := secondCampaign(t, h, "Saltmarsh")
	rec = call(h, http.MethodPost, "/api/v1/characters/"+ownedID+"/campaigns", "player", `{"campaignId":"`+second+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	joined := decode(t, rec)
	if joined["level"] != float64(1) || joined["hpCurrent"] != float64(12) || joined["name"] != "Kara" {
		t.Fatalf("joined sheet = %v", joined)
	}
	secondSheet, _ := joined["id"].(string)
	if rec := upload(h, "/api/v1/campaigns/"+first+"/characters/"+firstSheet+"/portrait", pngBytes); rec.Code != http.StatusNoContent {
		t.Fatalf("portrait: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+second+"/characters/"+secondSheet+"/portrait", "player", ""); rec.Code != http.StatusOK {
		t.Fatalf("the portrait in the second Campaign: %d", rec.Code)
	}
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+second+"/characters/"+secondSheet, "player", `{"hpCurrent":11}`); rec.Code != http.StatusOK {
		t.Fatalf("hurt in the second Campaign: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+first+"/characters/"+firstSheet+"/portrait", "player", ""); rec.Code != http.StatusOK {
		t.Fatalf("an edit elsewhere keeps the portrait: %d", rec.Code)
	}
	third := secondCampaign(t, h, "Phandalin")
	if rec := call(h, http.MethodPost, "/api/v1/characters/"+ownedID+"/campaigns", "player", `{"campaignId":"`+third+`"}`); rec.Code != http.StatusCreated {
		t.Fatalf("join a third: %d", rec.Code)
	}
	if !decode(t, call(h, http.MethodGet, "/api/v1/characters/"+ownedID, "player", ""))["hasPortrait"].(bool) {
		t.Fatal("the Character has no portrait")
	}
	if rec := call(h, http.MethodPost, "/api/v1/characters/"+ownedID+"/campaigns", "player", `{"campaignId":"`+second+`"}`); rec.Code != http.StatusConflict {
		t.Fatalf("joining twice: %d", rec.Code)
	}
	if hp := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+first+"/characters/"+firstSheet, "player", ""))["hpCurrent"]; hp != float64(5) {
		t.Fatalf("the first Campaign's hit points = %v", hp)
	}

	rec = call(h, http.MethodPut, "/api/v1/characters/"+ownedID, "player", `{"name":"Kara Vale","backstory":"  Raised by wolves.  "}`)
	if rec.Code != http.StatusOK || decode(t, rec)["backstory"] != "Raised by wolves." {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range []string{first, second} {
		if body := call(h, http.MethodGet, "/api/v1/campaigns/"+c+"/characters", "player", "").Body.String(); !strings.Contains(body, `"name":"Kara Vale"`) {
			t.Errorf("campaign %s = %s", c, body)
		}
	}
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+first+"/characters/"+firstSheet, "player", `{"name":"Kara of Morvain"}`); rec.Code != http.StatusOK {
		t.Fatalf("rename on a sheet: %d", rec.Code)
	}
	if name := decode(t, call(h, http.MethodGet, "/api/v1/characters/"+ownedID, "player", ""))["name"]; name != "Kara of Morvain" {
		t.Fatalf("a sheet's new name = %v", name)
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/characters/" + ownedID, ""},
		{http.MethodPut, "/api/v1/characters/" + ownedID, `{"name":"Stolen","backstory":""}`},
		{http.MethodPost, "/api/v1/characters/" + ownedID + "/campaigns", `{"campaignId":"` + first + `"}`},
	} {
		if rec := call(h, c.method, c.path, "dm", c.body); rec.Code != http.StatusNotFound {
			t.Errorf("someone else %s %s: %d", c.method, c.path, rec.Code)
		}
	}
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+first+"/characters/"+firstSheet, "stranger", `{"hpCurrent":1}`); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger edits the sheet: %d", rec.Code)
	}
	strangers := decode(t, call(h, http.MethodGet, "/api/v1/characters", "dm", ""))
	if items, _ := strangers["items"].([]any); len(items) != 0 {
		t.Fatalf("the DM's own Characters = %v", strangers)
	}
	elsewhere := call(h, http.MethodPost, "/api/v1/campaigns", "loner", `{"name":"Alone","displayName":"L"}`)
	lonely, _ := decode(t, elsewhere)["id"].(string)
	if rec := call(h, http.MethodPost, "/api/v1/characters/"+ownedID+"/campaigns", "player", `{"campaignId":"`+lonely+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("joining a Campaign the player is not in: %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, "/api/v1/characters/"+ownedID, "player", `{"name":"  ","backstory":""}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a blank name: %d", rec.Code)
	}
}
