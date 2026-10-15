package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
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
	lantern := `{"targeting":{"shape":"emanation","sizeFt":10,"rangeFt":0},"save":"wisdom","concentration":true,"duration":{"unit":"minutes","amount":1},
		"ritual":true,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false,"material":{"text":"a lantern of bog glass","costGp":25,"consumed":true,"item":"lantern"}},
		"parts":[{"type":"light","brightFt":20,"dimFt":20},{"type":"reveal","qualities":["invisible"]},{"type":"condition","condition":"charmed","onlyTypes":["undead","fey"]},
		{"type":"damage","when":"start_of_turn","dice":"1d6","damageType":"radiant"}]}`
	saved := decode(t, rec)
	sameJSON(t, "the saved design", saved["design"], lantern)
	sameJSON(t, "the saved text", saved["text"], `["Casting Time: 1 action or Ritual.","Components: V, M (a lantern of bog glass worth 25+ gp, which the spell consumes).",
		"Area: a 10-foot emanation from the caster.","Duration: Concentration, up to 1 minute.","Bright light fills 20 feet around the spell, and dim light another 20 feet.",
		"Every creature and object in the area loses Invisible.","A creature that fails the Wisdom saving throw has the Charmed condition, if it is Undead or Fey.",
		"A creature that starts its turn in the area takes 1d6 radiant damage."]`)
	sameJSON(t, "the design read back", decode(t, call(h, http.MethodGet, "/api/v1/builders/spells/"+spell, "dm", ""))["design"], lantern)

	old := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Burst","fields":[]}`))["id"].(string)
	if _, err := pool.Exec(context.Background(), "UPDATE library.entries SET design = $2 WHERE id = $1", old, burstRows); err != nil {
		t.Fatal(err)
	}
	stored := decode(t, call(h, http.MethodGet, "/api/v1/builders/spells/"+old, "dm", ""))
	sameJSON(t, "a stored design", stored["design"], burstRows)
	sameJSON(t, "a stored design's text", stored["text"], `["Casting Time: 1 action.","Components: V.","Area: a 20-foot sphere at a point within 150 feet.","Duration: 3 rounds.",
		"Each creature in the area takes 8d6 fire damage on a failed Dexterity saving throw, or half as much on a successful one.",
		"A creature that fails the Dexterity saving throw has the Prone condition.","The area is covered in grease for 3 rounds.","Flammable objects ignite."]`)

	frost := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"condition","name":"Frostbite","fields":[]}`))["id"].(string)
	cond := decode(t, call(h, http.MethodPut, "/api/v1/builders/conditions/"+frost, "dm", frostbiteDesign))
	sameJSON(t, "the saved condition", cond["design"], frostbiteDesign)
	sameJSON(t, "the condition's lines", cond["lines"], `["Cold seeps into the bones.","Ends on a rest.",
		"Each level gives a −1 penalty to D20 Tests and reduces Speed by 5 feet; it rises to level 3 at most.","Attack rolls against the target have Advantage.",
		"The target has Disadvantage on Dexterity saving throws."]`)
}

const burstRows = `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,
"duration":{"unit":"rounds","amount":3},"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
"parts":[{"type":"damage","when":"on_cast","dice":"8d6","damageType":"fire","half":true},{"type":"condition","condition":"prone"},
{"type":"surface","surface":"grease","rounds":3},{"type":"manual","text":"Flammable objects ignite."}]}`
