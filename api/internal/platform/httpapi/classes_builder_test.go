package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// lamplighterDesign casts from its own slot table off the bard's list: two 1st-level slots from level
// 1, a 2nd-level slot from level 4.
func lamplighterDesign(t *testing.T) string {
	t.Helper()
	twenty := func(n func(level int) int) []int {
		out := make([]int, 20)
		for l := range out {
			out[l] = n(l + 1)
		}
		return out
	}
	slots := make([][]int, 20)
	for l := range slots {
		slots[l] = make([]int, 9)
		slots[l][0] = 2
		if l >= 3 {
			slots[l][1] = 1
		}
	}
	marks := make([]string, 20)
	for l := range marks {
		marks[l] = "+1"
	}
	d := map[string]any{
		"hitDie": 8, "primary": []string{"strength"}, "saves": []string{"dexterity", "charisma"}, "armor": []string{"light"}, "weapons": []string{"simple"},
		"skills": 2, "subclassLevel": 3, "featLevels": []int{4, 8, 12, 16, 19},
		"columns":  []map[string]any{{"name": "Wick Marks", "values": marks}},
		"features": []map[string]any{{"level": 1, "name": "Wickcraft", "text": "You tend lanterns that burn without oil."}, {"level": 5, "name": "Bright Step", "text": "Teleport between lit lanterns."}},
		"casting": map[string]any{
			"kind": "slots", "ability": "charisma", "spellList": "bard", "cantrips": twenty(func(int) int { return 2 }),
			"prepared": twenty(func(l int) int { return 1 + l }), "slots": slots, "afterRest": true,
		},
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func classOffered(t *testing.T, h http.Handler, campaign, slug string) map[string]any {
	t.Helper()
	opts := decode(t, call(h, http.MethodGet, "/api/v1/compendium/builder?ruleset=srd-2024&campaignId="+campaign, "player", ""))
	for _, c := range opts["classes"].([]any) {
		if m := c.(map[string]any); m["slug"] == slug {
			return m
		}
	}
	return nil
}

// An author builds the Lamplighter in the class builder. A Campaign that links it offers it in character
// creation, where it gets its slots, features and training, and in multiclassing, where a fighter
// learns the bard's spells through it and keeps its slots apart.
func TestTheClassBuilderInCreationAndMulticlassing(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	design := lamplighterDesign(t)
	lamp := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"class","name":"Lamplighter","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/classes/" + lamp
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["design"].(map[string]any)["hitDie"] != float64(8) || !strings.HasPrefix(fresh["lines"].([]any)[1].(string), "No spellcasting") {
		t.Fatalf("a fresh class = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/classes/preview", "dm", `{"name":"Lamplighter","design":`+design+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "Level 4: Feat · Cantrips 2 · Prepared 5 · Slots 2/1 · Wick Marks +1") {
		t.Fatalf("preview: %d %s", rec.Code, lines)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/classes/preview", "dm", `{"name":"X","design":`+strings.Replace(design, `"bard"`, `"artificer"`, 1)+`}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "spell list") {
		t.Fatalf("an artificer's list: %d %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if rec := call(h, method, "/api/v1/builders/classes/"+npc, "dm", design); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an NPC in the class builder (%s): %d", method, rec.Code)
		}
	}
	if rec := call(h, http.MethodPut, path, "dm", strings.Replace(design, `"hitDie":8`, `"hitDie":7`, 1)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a d7: %d", rec.Code)
	}
	saved := decode(t, call(h, http.MethodPut, path, "dm", design))
	slug := saved["slug"].(string)
	if saved["entry"].(map[string]any)["revision"] != float64(2) {
		t.Fatalf("save = %v", saved)
	}
	if classOffered(t, h, campaign, slug) != nil {
		t.Fatal("a Campaign that never linked the Lamplighter does not offer it")
	}
	if rec := call(h, http.MethodGet, "/api/v1/compendium/builder?ruleset=srd-2024&campaignId="+campaign, "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a stranger's options: %d", rec.Code)
	}
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+lamp+`"}`)
	offered := classOffered(t, h, campaign, slug)
	if offered == nil || offered["name"] != "Lamplighter" || offered["caster"] != "slots" || offered["hitDie"] != float64(8) || offered["skillChoices"] != float64(2) {
		t.Fatalf("the Lamplighter offered = %v", offered)
	}

	base := "/api/v1/campaigns/" + campaign + "/characters"
	rec = call(h, http.MethodPost, base, "player", strings.Replace(srdFighter, `"class":"fighter"`, `"class":"`+slug+`"`, 1))
	created := decode(t, rec)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create a Lamplighter: %d %v", rec.Code, created)
	}
	lamplighter := decode(t, call(h, http.MethodGet, base+"/"+created["id"].(string), "player", ""))
	if !hasTrait(lamplighter, "Wickcraft") || hasTrait(lamplighter, "Bright Step") || lamplighter["hpMax"] != float64(10) || !strings.Contains(mustJSON(t, lamplighter["resources"]), `"key":"spell-slots-1"`) {
		t.Fatalf("a first-level Lamplighter = hp %v traits %v resources %v", lamplighter["hpMax"], lamplighter["traits"], lamplighter["resources"])
	}

	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	unlock(t, h, kara)
	plan := decode(t, call(h, http.MethodGet, kara+"/level-up?class="+slug, "player", ""))
	if plan["cantrips"] != float64(2) || plan["spells"] != float64(2) || plan["classLevel"] != float64(1) {
		t.Fatalf("a first Lamplighter level = %v", plan)
	}
	var cantrips, spells []string
	for _, s := range plan["spellList"].([]any) {
		sp := s.(map[string]any)
		if sp["level"] == float64(0) && len(cantrips) < 2 {
			cantrips = append(cantrips, sp["slug"].(string))
		} else if sp["level"] == float64(1) && len(spells) < 2 {
			spells = append(spells, sp["slug"].(string))
		}
	}
	if !strings.Contains(mustJSON(t, plan["spellList"]), `"vicious-mockery"`) {
		t.Fatalf("the Lamplighter learns from the bard's list: %v", plan["spellList"])
	}
	learned := mustJSON(t, append(cantrips, spells...))
	sheet := levelUp(t, h, kara, `{"class":"`+slug+`","spells":`+learned+`}`)
	resources := mustJSON(t, sheet["resources"])
	if len(sheet["classes"].([]any)) != 2 || !strings.Contains(resources, `"key":"`+slug+`-slots-1"`) || !hasTrait(sheet, "Wickcraft") {
		t.Fatalf("a fighter 1 / Lamplighter 1 = classes %v resources %s", sheet["classes"], resources)
	}
	cast := decode(t, call(h, http.MethodGet, kara+"/spells", "player", ""))
	if !strings.Contains(mustJSON(t, cast["classes"]), `"class":"`+slug+`"`) {
		t.Fatalf("the Lamplighter's spells = %v", cast)
	}
}
