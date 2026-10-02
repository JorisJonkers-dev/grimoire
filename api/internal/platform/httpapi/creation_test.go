package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// pointBuy is a point-buy build of 27 points with the background's bonuses.
const pointBuy = `{"name":"Kara","species":"human","class":"fighter","background":"soldier","method":"point-buy",
"base":{"strength":15,"dexterity":14,"constitution":13,"intelligence":8,"wisdom":10,"charisma":10},
"bonus":{"strength":1,"dexterity":1,"constitution":1},"skills":["perception","survival"],"shield":false,"weapons":["longsword"],
"appearance":"  Tall, scarred.  ","backstory":"Raised in the barracks."}`

// standardArray is a standard-array build.
const standardArray = `{"name":"Kara","species":"human","class":"fighter","background":"soldier","method":"standard-array",
"base":{"strength":15,"dexterity":14,"constitution":13,"intelligence":12,"wisdom":10,"charisma":8},
"bonus":{"strength":1,"dexterity":1,"constitution":1},"skills":["perception","survival"],"shield":false,"weapons":[]}`

func rolledBuild(base string) string {
	return `{"name":"Ros","species":"human","class":"fighter","background":"soldier","method":"rolled","base":` + base + `,
"bonus":{"strength":1,"dexterity":1,"constitution":1},"skills":["perception","survival"],"shield":false,"weapons":[]}`
}

// The DM chooses which ability score methods new Characters may use and the level they start at; the
// API holds every build to that, and rolled scores are rolled by the server, once per draft.
func TestCreationFollowsTheCampaign(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	for body, want := range map[string]int{
		`{"startingLevel":0}`:                                 http.StatusBadRequest,
		`{"creationMethods":[]}`:                              http.StatusBadRequest,
		`{"creationMethods":["point-buy","point-buy"]}`:       http.StatusBadRequest,
		`{"creationMethods":["point-buy"],"startingLevel":3}`: http.StatusOK,
	} {
		if rec := call(h, http.MethodPatch, base, "dm", body); rec.Code != want {
			t.Errorf("settings %s = %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPatch, base, "player", `{"startingLevel":5}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player changes the starting level: %d", rec.Code)
	}
	camp := decode(t, call(h, http.MethodGet, base, "player", ""))
	if camp["startingLevel"] != float64(3) || len(camp["creationMethods"].([]any)) != 1 {
		t.Fatalf("campaign = %v", camp)
	}
	if rec := call(h, http.MethodPost, base+"/characters", "player", standardArray); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "another way") {
		t.Fatalf("a method the Campaign does not allow: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(h, http.MethodPost, base+"/characters", "player", pointBuy)
	sheet := decode(t, rec)
	if rec.Code != http.StatusCreated || sheet["level"] != float64(3) || sheet["hpMax"] != float64(28) || !strings.Contains(rec.Body.String(), `"key":"hit-dice","label":"Hit Dice (d10)","current":3,"max":3`) {
		t.Fatalf("a third-level fighter: %d %v", rec.Code, sheet)
	}
	if rec := call(h, http.MethodPost, base+"/character-draft/roll", "player", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("rolling where the Campaign does not roll: %d", rec.Code)
	}
	owned := decode(t, call(h, http.MethodGet, "/api/v1/characters", "player", ""))["items"].([]any)[0].(map[string]any)
	if owned["backstory"] != "Raised in the barracks." {
		t.Fatalf("owned = %v", owned)
	}

	call(h, http.MethodPatch, base, "dm", `{"creationMethods":["rolled","point-buy"],"startingLevel":1}`)
	scores := `{"strength":16,"dexterity":15,"constitution":12,"intelligence":11,"wisdom":9,"charisma":8}`
	if rec := call(h, http.MethodPost, base+"/characters/preview", "dm", rolledBuild(scores)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "roll your ability scores first") {
		t.Fatalf("rolled scores never rolled: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/character-draft", "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no draft yet: %d", rec.Code)
	}
	rec = call(h, http.MethodPut, base+"/character-draft", "dm", `{"step":2,"build":{"name":"Ros","class":"fighter"}}`)
	if rec.Code != http.StatusOK || decode(t, rec)["step"] != float64(2) {
		t.Fatalf("save draft: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base+"/character-draft/roll", "dm", "")
	draft := decode(t, rec)
	if rec.Code != http.StatusOK || len(draft["rolled"].([]any)) != 6 || draft["build"].(map[string]any)["name"] != "Ros" {
		t.Fatalf("roll: %d %v", rec.Code, draft)
	}
	if rec := call(h, http.MethodPost, base+"/character-draft/roll", "dm", ""); rec.Code != http.StatusConflict {
		t.Fatalf("rolling twice: %d", rec.Code)
	}
	call(h, http.MethodPut, base+"/character-draft", "dm", `{"step":3,"build":{"name":"Ros"}}`)
	if got := decode(t, call(h, http.MethodGet, base+"/character-draft", "dm", "")); len(got["rolled"].([]any)) != 6 || got["step"] != float64(3) {
		t.Fatalf("saving keeps the rolled scores = %v", got)
	}
	wrong := `{"strength":18,"dexterity":15,"constitution":12,"intelligence":11,"wisdom":9,"charisma":8}`
	if rec := call(h, http.MethodPost, base+"/characters", "dm", rolledBuild(wrong)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "place the six scores") {
		t.Fatalf("scores not rolled: %d %s", rec.Code, rec.Body.String())
	}
	placed := `{"strength":15,"dexterity":16,"constitution":12,"intelligence":8,"wisdom":9,"charisma":11}`
	if rec := call(h, http.MethodPost, base+"/characters", "dm", rolledBuild(placed)); rec.Code != http.StatusCreated {
		t.Fatalf("rolled scores placed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/character-draft", "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the draft goes once the Character is made: %d", rec.Code)
	}
	call(h, http.MethodPut, base+"/character-draft", "dm", `{"step":1,"build":{}}`)
	if rec := call(h, http.MethodDelete, base+"/character-draft", "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("discard: %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, base+"/character-draft", "stranger", `{"step":1,"build":{}}`); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger drafts: %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, base+"/character-draft", "dm", `{"step":9,"build":{}}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a step past the review: %d", rec.Code)
	}
}

// A Character joining another Campaign starts at that Campaign's starting level.
func TestJoiningStartsAtTheCampaignsLevel(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	first, _ := campaignWithPlayer(t, h)
	call(h, http.MethodPost, "/api/v1/campaigns/"+first+"/characters", "player", build)
	ownedID, _ := decode(t, call(h, http.MethodGet, "/api/v1/characters", "player", ""))["items"].([]any)[0].(map[string]any)["id"].(string)
	second := secondCampaign(t, h, "Saltmarsh")
	call(h, http.MethodPatch, "/api/v1/campaigns/"+second, "dm2", `{"startingLevel":5,"creationMethods":["rolled"]}`)
	rec := call(h, http.MethodPost, "/api/v1/characters/"+ownedID+"/campaigns", "player", `{"campaignId":"`+second+`"}`)
	if rec.Code != http.StatusCreated || decode(t, rec)["level"] != float64(5) {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
}
