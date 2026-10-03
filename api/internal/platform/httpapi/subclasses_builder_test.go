package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const wardenDesign = `{"class":"fighter",
"features":[{"level":3,"name":"Lantern Oath","text":"You carry a lantern that never gutters."},
{"level":3,"name":"Kindle","text":"As a Bonus Action you brighten the lantern.","uses":"light","spell":"light","spellName":"Light"},
{"level":7,"name":"Steady Flame","text":"Your lantern burns through magical darkness."}],
"resources":[{"key":"light","name":"Lantern Light","basis":"proficiency","fromLevel":3,"recharge":"long_rest"}],
"choices":[{"level":3,"name":"Lantern Style","count":1,"options":["Bog Glass","Ember Wick"]}]}`

// An author builds the Lantern Warden in the subclass builder; once a Campaign links it, a fighter
// reaching level 3 is offered it, picks its Lantern Style and gains its level 3 features.
func TestTheSubclassBuilderAndTheLevelUpWizard(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + campaign + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	warden := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"subclass","name":"Lantern Warden","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/subclasses/" + warden
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["design"].(map[string]any)["class"] != "fighter" || fresh["lines"].([]any)[0] != "Fighter subclass" {
		t.Fatalf("a fresh subclass = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/subclasses/preview", "dm", `{"name":"Lantern Warden","design":`+wardenDesign+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "Level 3 choice: Lantern Style, pick 1 of Bog Glass, Ember Wick.") {
		t.Fatalf("preview: %d %s", rec.Code, lines)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/subclasses/preview", "dm", `{"name":"X","design":`+strings.Replace(wardenDesign, `"fighter"`, `"artificer"`, 1)+`}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "choose the class") {
		t.Fatalf("an artificer subclass: %d %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if rec := call(h, method, "/api/v1/builders/subclasses/"+npc, "dm", wardenDesign); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "opens only entries of kind subclass") {
			t.Fatalf("an NPC in the subclass builder (%s): %d", method, rec.Code)
		}
	}
	if rec := call(h, http.MethodPut, path, "player", wardenDesign); rec.Code != http.StatusNotFound {
		t.Fatalf("a player saves the DM's subclass: %d", rec.Code)
	}
	if rec := call(h, http.MethodPut, path, "dm", strings.Replace(wardenDesign, `"uses":"light"`, `"uses":"dark"`, 1)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a feature spending a Resource it lacks: %d", rec.Code)
	}
	rec = call(h, http.MethodPut, path, "dm", wardenDesign)
	saved := decode(t, rec)
	if rec.Code != http.StatusOK || saved["entry"].(map[string]any)["revision"] != float64(2) {
		t.Fatalf("save: %d %v", rec.Code, saved)
	}
	slug := saved["slug"].(string)

	unlock(t, h, kara)
	levelUp(t, h, kara, `{"class":"fighter","hitPoints":"average"}`)
	unlock(t, h, kara)
	if _, ok := choiceOptions(decode(t, call(h, http.MethodGet, kara+"/level-up", "player", "")), "subclass")[slug]; ok {
		t.Fatal("a Campaign that never linked the Lantern Warden does not offer it")
	}
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+warden+`"}`)
	plan := decode(t, call(h, http.MethodGet, kara+"/level-up", "player", ""))
	if opt, ok := choiceOptions(plan, "subclass")[slug]; !ok || opt["name"] != "Lantern Warden" {
		t.Fatalf("level 3 subclasses = %v", choiceOptions(plan, "subclass"))
	}
	style := slug + "-lantern-style"
	if len(choiceOptions(plan, style)) != 0 {
		t.Fatal("the Lantern Style waits for the Lantern Warden to be picked")
	}
	plan = decode(t, call(h, http.MethodGet, kara+"/level-up?subclass="+slug, "player", ""))
	if opts := choiceOptions(plan, style); opts["bog-glass"]["name"] != "Bog Glass" || len(opts) != 2 {
		t.Fatalf("the Lantern Style = %v", plan["choices"])
	}
	if opts := choiceOptions(decode(t, call(h, http.MethodGet, kara+"/level-up?subclass=champion-of-nothing", "player", "")), style); len(opts) != 0 {
		t.Fatal("a subclass the class does not offer adds no choices")
	}
	if rec := call(h, http.MethodPost, kara+"/level-up", "player", `{"class":"fighter","picks":[{"choice":"subclass","values":["`+slug+`"]}]}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Lantern Style") {
		t.Fatalf("skipping the Lantern Style: %d %s", rec.Code, rec.Body.String())
	}
	sheet := levelUp(t, h, kara, `{"class":"fighter","picks":[{"choice":"subclass","values":["`+slug+`"]},{"choice":"`+style+`","values":["ember-wick"]}]}`)
	fighter := sheet["classes"].([]any)[0].(map[string]any)
	if fighter["subclass"] != slug || !hasTrait(sheet, "Lantern Oath") || !hasTrait(sheet, "Kindle") || hasTrait(sheet, "Steady Flame") {
		t.Fatalf("level 3 = %v traits %v", fighter, sheet["traits"])
	}
}
