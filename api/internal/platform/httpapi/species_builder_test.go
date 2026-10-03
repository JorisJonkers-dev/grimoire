package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const marshkinDesign = `{"sizes":["small","medium"],"creatureType":"humanoid","speedFt":30,"speeds":[{"kind":"swim","feet":30}],
"senses":[{"kind":"darkvision","feet":60}],"resistances":["poison"],"traits":[{"name":"Reed Breath","text":"You can hold your breath for an hour."}],
"spells":[{"level":3,"spell":"fog-cloud","name":"Fog Cloud","uses":"long_rest"}],
"lineages":[{"name":"Bog","text":"You know the Druidcraft cantrip.","spells":[{"level":1,"spell":"druidcraft","name":"Druidcraft","uses":"at_will"}]},
{"name":"Reed","text":"Your Speed rises by 5 feet.","spells":[]}]}`

func speciesOffered(t *testing.T, h http.Handler, campaign string) map[string]map[string]any {
	t.Helper()
	opts := decode(t, call(h, http.MethodGet, "/api/v1/compendium/builder?ruleset=srd-2024&campaignId="+campaign, "player", ""))
	out := map[string]map[string]any{}
	for _, s := range opts["species"].([]any) {
		m := s.(map[string]any)
		out[m["slug"].(string)] = m
	}
	return out
}

// An author builds the Marshkin in the species builder. A Campaign that links it offers each lineage in
// character creation, and a Bog Marshkin's sheet carries its traits, its lineage's cantrip, and the
// species's spell only once it reaches level 3.
func TestTheSpeciesBuilderInCreation(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	marsh := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"species","name":"Marshkin","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/species/" + marsh
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["design"].(map[string]any)["speedFt"] != float64(30) || fresh["lines"].([]any)[0] != "Marshkin: 30 feet." {
		t.Fatalf("a fresh species = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/species/preview", "dm", `{"name":"Marshkin","design":`+marshkinDesign+`}`)
	if lines := strings.Join(toStrings(decode(t, rec)["lines"].([]any)), "\n"); rec.Code != http.StatusOK || !strings.Contains(lines, "Marshkin, Reed lineage: 30 feet; Swim 30 feet.") {
		t.Fatalf("preview: %d %s", rec.Code, lines)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/species/preview", "dm", `{"name":"X","design":`+strings.Replace(marshkinDesign, `"humanoid"`, `"robot"`, 1)+`}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "creature type") {
		t.Fatalf("a robot: %d %s", rec.Code, rec.Body.String())
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if rec := call(h, method, "/api/v1/builders/species/"+npc, "dm", marshkinDesign); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an NPC in the species builder (%s): %d", method, rec.Code)
		}
	}
	if rec := call(h, http.MethodPut, path, "dm", strings.Replace(marshkinDesign, `"speedFt":30`, `"speedFt":90`, 1)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a Speed of 90: %d", rec.Code)
	}
	saved := decode(t, call(h, http.MethodPut, path, "dm", marshkinDesign))
	bog := saved["slug"].(string) + "-bog"
	if _, ok := speciesOffered(t, h, campaign)[bog]; ok {
		t.Fatal("a Campaign that never linked the Marshkin does not offer it")
	}
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+marsh+`"}`)
	offered := speciesOffered(t, h, campaign)
	if offered[bog]["name"] != "Marshkin, Bog lineage" || offered[bog]["speedFeet"] != float64(30) || len(offered) < 3 {
		t.Fatalf("species offered = %v", offered)
	}

	base := "/api/v1/campaigns/" + campaign + "/characters"
	created := decode(t, call(h, http.MethodPost, base, "player", strings.Replace(srdFighter, `"species":"human"`, `"species":"`+bog+`"`, 1)))
	kara := base + "/" + created["id"].(string)
	sheet := decode(t, call(h, http.MethodGet, kara, "player", ""))
	if !hasTrait(sheet, "Bog lineage") || !hasTrait(sheet, "Druidcraft") || !hasTrait(sheet, "Size") || hasTrait(sheet, "Fog Cloud") || hasTrait(sheet, "Resourceful") {
		t.Fatalf("a Bog Marshkin at level 1 = %v", sheet["traits"])
	}
	for _, body := range []string{`{"class":"fighter"}`, `{"class":"fighter","picks":[{"choice":"subclass","values":["champion"]}]}`} {
		unlock(t, h, kara)
		levelUp(t, h, kara, body)
	}
	if sheet := decode(t, call(h, http.MethodGet, kara, "player", "")); !hasTrait(sheet, "Fog Cloud") {
		t.Fatalf("a Bog Marshkin at level 3 = %v", sheet["traits"])
	}
}
