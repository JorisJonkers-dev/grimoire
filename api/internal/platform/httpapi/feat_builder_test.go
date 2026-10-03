package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const lanternFeatDesign = `{"category":"general","text":"Your lanterns burn twice as long.","repeatable":true,
"prerequisites":[{"kind":"level","minimum":4,"group":0}]}`

const bogwardenDesign = `{"abilities":["strength","wisdom","constitution"],"skills":["survival","nature"],"feat":"alert","featName":"Alert",
"tool":"Herbalism Kit","equipment":"A lantern, a pole and 8 GP","gold":50,"text":"You kept the causeways open through the fens."}`

// An author builds the Bogwarden background and the Lampwright feat. A Campaign that links them offers
// the Bogwarden in character creation, where its Origin feat and grants reach the sheet, and the
// Lampwright, which repeats, whenever a level grants a General feat from level 4.
func TestTheBackgroundAndFeatBuilders(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	feat := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"feat","name":"Lampwright","fields":[]}`))["id"].(string)
	background := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"background","name":"Bogwarden","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	if fresh := decode(t, call(h, http.MethodGet, "/api/v1/builders/feats/"+feat, "dm", "")); fresh["lines"].([]any)[0] != "General feat" {
		t.Fatalf("a fresh feat = %v", fresh)
	}
	if fresh := decode(t, call(h, http.MethodGet, "/api/v1/builders/backgrounds/"+background, "dm", "")); fresh["design"].(map[string]any)["feat"] != "alert" {
		t.Fatalf("a fresh background = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/feats/preview", "dm", `{"name":"Lampwright","design":`+lanternFeatDesign+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "General feat, repeatable\nPrerequisites: Level 4+") {
		t.Fatalf("feat preview: %d %s", rec.Code, lines)
	}
	rec = call(h, http.MethodPost, "/api/v1/builders/backgrounds/preview", "dm", `{"name":"Bogwarden","design":`+bogwardenDesign+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "Skill Proficiencies: Survival, Nature") {
		t.Fatalf("background preview: %d %s", rec.Code, lines)
	}
	for path, body := range map[string]string{
		"/api/v1/builders/feats/preview":       `{"name":"X","design":` + strings.Replace(lanternFeatDesign, `"general"`, `"heroic"`, 1) + `}`,
		"/api/v1/builders/backgrounds/preview": `{"name":"X","design":` + strings.Replace(bogwardenDesign, `"nature"`, `"juggling"`, 1) + `}`,
	} {
		if rec := call(h, http.MethodPost, path, "dm", body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	for _, path := range []string{"/api/v1/builders/feats/" + npc, "/api/v1/builders/backgrounds/" + npc} {
		if rec := call(h, http.MethodGet, path, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an NPC in %s: %d", path, rec.Code)
		}
	}
	featSlug := decode(t, call(h, http.MethodPut, "/api/v1/builders/feats/"+feat, "dm", lanternFeatDesign))["slug"].(string)
	backgroundSlug := decode(t, call(h, http.MethodPut, "/api/v1/builders/backgrounds/"+background, "dm", bogwardenDesign))["slug"].(string)
	for _, id := range []string{feat, background} {
		call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+id+`"}`)
	}
	opts := decode(t, call(h, http.MethodGet, "/api/v1/compendium/builder?ruleset=srd-2024&campaignId="+campaign, "player", ""))
	if !strings.Contains(mustJSON(t, opts["backgrounds"]), `"slug":"`+backgroundSlug+`"`) {
		t.Fatalf("backgrounds offered = %v", opts["backgrounds"])
	}

	base := "/api/v1/campaigns/" + campaign + "/characters"
	build := strings.NewReplacer(`"background":"soldier"`, `"background":"`+backgroundSlug+`"`, `"bonus":{"strength":1,"dexterity":1,"constitution":1}`, `"bonus":{"strength":2,"constitution":1}`,
		`"skills":["perception","survival"]`, `"skills":["perception","athletics"]`).Replace(srdFighter)
	rec = call(h, http.MethodPost, base, "player", build)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create a Bogwarden: %d %s", rec.Code, rec.Body.String())
	}
	kara := base + "/" + decode(t, rec)["id"].(string)
	sheet := decode(t, call(h, http.MethodGet, kara, "player", ""))
	if !hasTrait(sheet, "Origin Feat") || !hasTrait(sheet, "Tool Proficiency") || !hasTrait(sheet, "Alert") || !hasTrait(sheet, "Bogwarden") {
		t.Fatalf("a Bogwarden's traits = %v", sheet["traits"])
	}
	steps := []string{
		`{"class":"fighter"}`,
		`{"class":"fighter","picks":[{"choice":"subclass","values":["champion"]}]}`,
		`{"class":"fighter","picks":[{"choice":"feat","values":["` + featSlug + `"]}]}`,
		`{"class":"fighter"}`,
	}
	for i, body := range steps {
		unlock(t, h, kara)
		if i == 2 {
			if opt := choiceOptions(decode(t, call(h, http.MethodGet, kara+"/level-up", "player", "")), "feat")[featSlug]; opt == nil || len(opt["unmet"].([]any)) != 0 {
				t.Fatalf("the Lampwright at level 4 = %v", opt)
			}
		}
		levelUp(t, h, kara, body)
	}
	unlock(t, h, kara)
	if _, ok := choiceOptions(decode(t, call(h, http.MethodGet, kara+"/level-up", "player", "")), "feat")[featSlug]; !ok {
		t.Fatal("the Lampwright repeats at level 6")
	}
	if sheet := decode(t, call(h, http.MethodGet, kara, "player", "")); !hasTrait(sheet, "Lampwright") {
		t.Fatalf("a Lampwright's traits = %v", sheet["traits"])
	}
}
