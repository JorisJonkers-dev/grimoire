package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// fighterAt4 is Kara levelled to 4: a Champion with an Ability Score Improvement to Strength.
func fighterAt4(t *testing.T, h http.Handler, base string) string {
	t.Helper()
	path := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	for _, body := range []string{
		`{"class":"fighter"}`,
		`{"class":"fighter","picks":[{"choice":"subclass","values":["champion"]}]}`,
		`{"class":"fighter","picks":[{"choice":"feat","values":["ability-score-improvement"]}],"increase":{"strength":2}}`,
	} {
		unlock(t, h, path)
		levelUp(t, h, path, body)
	}
	return path
}

func retrainBody(build string) string {
	return `{"reason":"Grappling suits her better","build":` + build + `}`
}

const sageBuild = `{"species":"human","background":"sage","method":"point-buy",
"base":{"strength":15,"dexterity":12,"constitution":15,"intelligence":8,"wisdom":10,"charisma":8},
"bonus":{"constitution":2,"intelligence":1},"increase":{"strength":0},"skills":["perception","survival"],
"picks":[{"level":4,"choice":"feat","value":"grappler"}]}`

// A player asks to rebuild a Character; the DM approves it, the old build is kept as a Revision, and
// the sheet follows the new build, hit points included. Rebuilds that break the rules never reach the DM.
func TestRetrainWithDMApproval(t *testing.T) {
	t.Parallel()
	h := srdCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	path := fighterAt4(t, h, "/api/v1/campaigns/"+id+"/characters")
	before := decode(t, call(h, http.MethodGet, path, "player", ""))
	if !featTrait(before, "Ability Score Improvement") {
		t.Fatal("the feat taken at level 4 is not on the sheet")
	}

	choices := decodeList(t, call(h, http.MethodGet, path+"/retrains/choices", "player", ""))
	if len(choices) != 1 || choices[0]["choice"] != "feat" || choices[0]["value"] != "ability-score-improvement" || len(choices[0]["options"].([]any)) != 2 {
		t.Fatalf("retrain choices = %v", choices)
	}
	if rec := call(h, http.MethodGet, path+"/retrains/choices", "dm", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("the DM listing a player's retrain choices: %d", rec.Code)
	}
	for build, why := range map[string]string{
		strings.Replace(sageBuild, `"picks":[{"level":4,"choice":"feat","value":"grappler"}]`, `"picks":[]`, 1): "a dropped pick",
		strings.Replace(sageBuild, `"value":"grappler"`, `"value":"ability-score-improvement"`, 1):              "an improvement without points",
		strings.Replace(sageBuild, `"value":"grappler"`, `"value":"archery"`, 1):                                "a feat from another category",
		strings.Replace(sageBuild, `"background":"sage"`, `"background":"pirate"`, 1):                           "an unknown background",
	} {
		if rec := call(h, http.MethodPost, path+"/retrains", "player", retrainBody(build)); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", why, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPost, path+"/retrains", "dm", retrainBody(sageBuild)); rec.Code != http.StatusForbidden {
		t.Fatalf("the DM asking for a player's retrain: %d", rec.Code)
	}
	rec := call(h, http.MethodPost, path+"/retrains", "player", retrainBody(sageBuild))
	if rec.Code != http.StatusCreated {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	retrain := decode(t, rec)
	if retrain["status"] != "pending" || retrain["proposed"].(map[string]any)["background"] != "sage" {
		t.Fatalf("retrain = %v", retrain)
	}
	if rec := call(h, http.MethodPost, path+"/retrains", "player", retrainBody(sageBuild)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a second pending retrain: %d", rec.Code)
	}
	if list := decodeList(t, call(h, http.MethodGet, path+"/retrains", "dm", "")); len(list) != 1 {
		t.Fatalf("retrains = %v", list)
	}
	decide := "/api/v1/campaigns/" + id + "/retrains/" + retrain["id"].(string)
	if rec := call(h, http.MethodPost, decide+"/approve", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a player approving: %d", rec.Code)
	}
	if approved := decode(t, call(h, http.MethodPost, decide+"/approve", "dm", "")); approved["status"] != "approved" || approved["decidedBy"] != "Joris" {
		t.Fatalf("approved = %v", approved)
	}
	if rec := call(h, http.MethodPost, decide+"/decline", "dm", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("deciding twice: %d", rec.Code)
	}

	after := decode(t, call(h, http.MethodGet, path, "player", ""))
	if after["background"].(map[string]any)["slug"] != "sage" || score(after, "strength") != 15 || score(after, "constitution") != 17 ||
		after["hpMax"] != before["hpMax"].(float64)+4 || !featTrait(after, "Grappler") || featTrait(after, "Ability Score Improvement") {
		t.Fatalf("retrained sheet: background %v str %v con %v hp %v -> %v", after["background"], score(after, "strength"), score(after, "constitution"), before["hpMax"], after["hpMax"])
	}
	revisions := decodeList(t, call(h, http.MethodGet, path+"/revisions", "player", ""))
	if len(revisions) != 1 || revisions[0]["no"] != float64(1) || revisions[0]["retrainId"] != retrain["id"] {
		t.Fatalf("revisions = %v", revisions)
	}
	kept := revisions[0]["build"].(map[string]any)
	if kept["background"] != "soldier" || kept["increase"].(map[string]any)["strength"] != float64(2) || kept["picks"].([]any)[0].(map[string]any)["value"] != "ability-score-improvement" {
		t.Fatalf("kept build = %v", kept)
	}

	back := strings.NewReplacer(`"background":"sage"`, `"background":"soldier"`, `"bonus":{"constitution":2,"intelligence":1}`, `"bonus":{"strength":1,"dexterity":1,"constitution":1}`).Replace(sageBuild)
	second := decode(t, call(h, http.MethodPost, path+"/retrains", "player", retrainBody(back)))
	if declined := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/retrains/"+second["id"].(string)+"/decline", "dm", "")); declined["status"] != "declined" {
		t.Fatalf("declined = %v", declined)
	}
	if list := decodeList(t, call(h, http.MethodGet, path+"/retrains", "player", "")); len(list) != 2 || list[0]["status"] != "declined" {
		t.Fatalf("history = %v", list)
	}
	if rec := call(h, http.MethodGet, path+"/revisions", "stranger", ""); rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("a stranger reading Revisions: %d", rec.Code)
	}
}

func featTrait(sheet map[string]any, name string) bool {
	for _, t := range sheet["traits"].([]any) {
		if tr := t.(map[string]any); tr["name"] == name && tr["source"] == "feat" {
			return true
		}
	}
	return false
}
