package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const frostbiteDesign = `{"icon":"snow","color":"#7fa8dd","text":"Cold seeps into the bones.","ends":"rest","stacks":true,"maxLevel":3,
"perLevel":{"d20":1,"speedFt":5,"deathAt":0},"parts":[{"type":"attacked_advantage"},{"type":"save_disadvantage","ability":"dexterity"}]}`

// An author builds Frostbite in the condition builder, and a DM picks the Campaign's exhaustion.
func TestTheConditionBuilderAndExhaustionVariants(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	frost := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"condition","name":"Frostbite","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/conditions/" + frost
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["design"].(map[string]any)["icon"] != "spiral" || fresh["lines"].([]any)[0] != "Lasts until removed." {
		t.Fatalf("a fresh condition = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/conditions/preview", "dm", `{"name":"Frostbite","design":`+frostbiteDesign+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "it rises to level 3 at most") {
		t.Fatalf("preview: %d %s", rec.Code, lines)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/conditions/preview", "dm", `{"name":"X","design":`+strings.Replace(frostbiteDesign, `"snow"`, `"rocket"`, 1)+`}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a rocket icon: %d", rec.Code)
	}
	// A lingering injury names its cure and reads back as lasting until it; without a cure it does not build.
	limp := `{"icon":"skull","color":"#aa3344","text":"","ends":"cure","cure":"Regenerate","perLevel":{"d20":0,"speedFt":0,"deathAt":0},"parts":[]}`
	rec = call(h, http.MethodPost, "/api/v1/builders/conditions/preview", "dm", `{"name":"Limp","design":`+limp+`}`)
	if got := decode(t, rec); rec.Code != http.StatusOK || got["design"].(map[string]any)["cure"] != "Regenerate" || !strings.Contains(strings.Join(toStrings(got["lines"].([]any)), "\n"), "Lingers through every rest until cured: Regenerate.") {
		t.Fatalf("a lingering injury: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/conditions/preview", "dm", `{"name":"Limp","design":`+strings.Replace(limp, `"cure":"Regenerate",`, "", 1)+`}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "name its cure") {
		t.Fatalf("a lingering injury with no cure: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/builders/conditions/"+npc, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an NPC in the condition builder: %d", rec.Code)
	}
	if saved := decode(t, call(h, http.MethodPut, path, "dm", frostbiteDesign)); saved["entry"].(map[string]any)["revision"] != float64(2) {
		t.Fatalf("save = %v", saved)
	}

	settings := "/api/v1/campaigns/" + campaign
	if got := decode(t, call(h, http.MethodGet, settings, "dm", "")); got["exhaustion"] != "srd-2024" {
		t.Fatalf("a new Campaign's exhaustion = %v", got["exhaustion"])
	}
	if rec := call(h, http.MethodPatch, settings, "dm", `{"exhaustion":"brutal"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a brutal exhaustion: %d", rec.Code)
	}
	if rec := call(h, http.MethodPatch, settings, "player", `{"exhaustion":"grim"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player picking the exhaustion: %d", rec.Code)
	}
	if got := decode(t, call(h, http.MethodPatch, settings, "dm", `{"exhaustion":"grim"}`)); got["exhaustion"] != "grim" {
		t.Fatalf("grim exhaustion = %v", got)
	}
}
