package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// sameJSON fails unless got holds the same JSON value as want.
func sameJSON(t *testing.T, what string, got any, want string) {
	t.Helper()
	var w any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, w) {
		t.Errorf("%s =\n%s\nwant\n%s", what, mustJSON(t, got), mustJSON(t, w))
	}
}

// What a homebrew spell and condition look like on the wire as they are saved and read back, and as a
// design stored before reads back.
func TestHomebrewDesignsOnTheWire(t *testing.T) {
	t.Parallel()
	h, pool := armouryStack(t)
	spell := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Marsh Lantern","fields":[]}`))["id"].(string)
	rec := call(h, http.MethodPut, "/api/v1/builders/spells/"+spell, "dm", lanternDesign)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	charm := func(creature string) string {
		return `{"kind":"branch","values":{"when":{"text":"creature_is"},"creature_type":{"text":"` + creature + `"}},"groups":[{"name":"then","parts":[
			{"kind":"branch","values":{"when":{"text":"fails_by"},"n":{"number":1}},"groups":[{"name":"then","parts":[
			{"kind":"save_condition","values":{"condition":{"text":"charmed"}},"groups":[]}]}]}]}]}`
	}
	lantern := `{"targeting":{"shape":"emanation","sizeFt":10,"rangeFt":0},"save":"wisdom","concentration":true,"duration":{"unit":"minutes","amount":1},
		"ritual":true,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false,"material":{"text":"a lantern of bog glass","costGp":25,"consumed":true,"item":"lantern"}},
		"turnDamage":{"dice":"1d6","damageType":"radiant"},
		"parts":[{"kind":"light","values":{"bright_ft":{"number":20},"dim_ft":{"number":20}},"groups":[]},{"kind":"reveal","values":{"qualities":{"texts":["invisible"]}},"groups":[]},
		` + charm("undead") + `,` + charm("fey") + `]}`
	saved := decode(t, rec)
	sameJSON(t, "the saved design", saved["design"], lantern)
	sameJSON(t, "the saved text", saved["text"], `["Casting Time: 1 action or Ritual.","Components: V, M (a lantern of bog glass worth 25+ gp, which the spell consumes).",
		"Area: a 10-foot emanation from the caster.","Duration: Concentration, up to 1 minute.","Bright light fills 20 feet around where it lands, and dim light another 20 feet.",
		"Every creature and object in the area loses Invisible, whatever anyone's senses.",
		"If the creature is an undead, if a creature fails the save by 1 or more, on a failed Wisdom saving throw, a creature has the Charmed condition.",
		"If the creature is a fey, if a creature fails the save by 1 or more, on a failed Wisdom saving throw, a creature has the Charmed condition.",
		"A creature that starts its turn in the area takes 1d6 radiant damage."]`)
	sameJSON(t, "the design read back", decode(t, call(h, http.MethodGet, "/api/v1/builders/spells/"+spell, "dm", ""))["design"], lantern)

	old := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Burst","fields":[]}`))["id"].(string)
	if _, err := pool.Exec(context.Background(), "UPDATE library.entries SET design = $2 WHERE id = $1", old, burstRows); err != nil {
		t.Fatal(err)
	}
	stored := decode(t, call(h, http.MethodGet, "/api/v1/builders/spells/"+old, "dm", ""))
	sameJSON(t, "a stored design", stored["design"], `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,
		"duration":{"unit":"rounds","amount":3},"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
		"parts":[{"kind":"save_damage","values":{"dice":{"text":"8d6"},"damage_type":{"text":"fire"},"half":{"flag":true}},"groups":[]},
		{"kind":"branch","values":{"when":{"text":"fails_by"},"n":{"number":1}},"groups":[{"name":"then","parts":[{"kind":"save_condition","values":{"condition":{"text":"prone"}},"groups":[]}]}]},
		{"kind":"create_surface","values":{"surface":{"text":"grease"},"rounds":{"number":3}},"groups":[]},
		{"kind":"manual","values":{"instruction":{"text":"Flammable objects ignite."}},"groups":[]}]}`)
	sameJSON(t, "a stored design's text", stored["text"], `["Casting Time: 1 action.","Components: V.","Area: a 20-foot sphere at a point within 150 feet.","Duration: 3 rounds.",
		"Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.",
		"If a creature fails the save by 1 or more, on a failed Dexterity saving throw, a creature has the Prone condition.","The area is covered in grease for 3 rounds.","Flammable objects ignite."]`)

	frost := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"condition","name":"Frostbite","fields":[]}`))["id"].(string)
	cond := decode(t, call(h, http.MethodPut, "/api/v1/builders/conditions/"+frost, "dm", frostbiteDesign))
	sameJSON(t, "the saved condition", cond["design"], `{"icon":"snow","color":"#7fa8dd","text":"Cold seeps into the bones.","ends":"rest","stacks":true,"maxLevel":3,
		"perLevel":{"d20":1,"speedFt":5,"deathAt":0},"parts":[{"kind":"edge","values":{"against":{"flag":true},"advantage":{"flag":true},"range":{"text":"any"}},"groups":[]},
		{"kind":"save_edge","values":{"ability":{"text":"dexterity"},"mode":{"text":"disadvantage"}},"groups":[]}]}`)
	sameJSON(t, "the condition's lines", cond["lines"], `["Cold seeps into the bones.","Ends on a rest.",
		"Each level gives a −1 penalty to D20 Tests and reduces Speed by 5 feet; it rises to level 3 at most.","Attack rolls against the target have Advantage.",
		"The target has Disadvantage on Dexterity saving throws."]`)
}

const burstRows = `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,
"duration":{"unit":"rounds","amount":3},"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
"parts":[{"type":"damage","when":"on_cast","dice":"8d6","damageType":"fire","half":true},{"type":"condition","condition":"prone"},
{"type":"surface","surface":"grease","rounds":3},{"type":"manual","text":"Flammable objects ignite."}]}`

// A Part its kind refuses is answered with the field and the reason from the kind's own checks; the rows
// of the builders' own part types are no longer a shape the API takes.
func TestPartsAreCheckedByTheirKinds(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	spell := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Burst","fields":[]}`))["id"].(string)
	design := func(parts string) string {
		return `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,"duration":{"unit":"instant"},
			"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},"parts":[` + parts + `]}`
	}
	for parts, want := range map[string]string{
		`{"kind":"save_damage","values":{"dice":{"text":"8d7"},"damage_type":{"text":"fire"}}}`:                                             "part 1: dice: write the damage like 8d6",
		`{"kind":"branch","values":{"when":{"text":"first_each_turn"}},"groups":[{"name":"then","parts":[{"kind":"manual","values":{}}]}]}`: "part 1: then: part 1: instruction: write up to 500 letters",
		`{"kind":"bonus_die","values":{"dice":{"text":"1d4"}}}`:                                                                             "a spell holds Parts of the kinds",
	} {
		rec := call(h, http.MethodPut, "/api/v1/builders/spells/"+spell, "dm", design(parts))
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %s", parts, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPut, "/api/v1/builders/spells/"+spell, "dm", design(`{"type":"manual","text":"Ring."}`)); rec.Code != http.StatusBadRequest {
		t.Fatalf("a row of the builder's own part types: %d %s", rec.Code, rec.Body.String())
	}
	cond := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"condition","name":"Dazed","fields":[]}`))["id"].(string)
	rec := call(h, http.MethodPut, "/api/v1/builders/conditions/"+cond, "dm", `{"icon":"snow","color":"#7fa8dd","text":"","ends":"rest",
		"perLevel":{"d20":0,"speedFt":0,"deathAt":0},"parts":[{"kind":"speed_penalty","values":{"ft":{"number":125}}}]}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "part 1: ft: runs from 0 to 120") {
		t.Fatalf("a condition's Part its kind refuses: %d %s", rec.Code, rec.Body.String())
	}
}
