package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const bogKingDesign = `{"size":"large","creatureType":"monstrosity","ac":16,"hp":120,"speedFt":30,"challenge":8,
"abilities":{"strength":20,"dexterity":12,"constitution":18,"intelligence":8,"wisdom":14,"charisma":16},
"saves":["constitution","wisdom"],"senses":[{"kind":"darkvision","feet":60}],"resistances":["poison"],"immunities":["acid"],"vulnerabilities":["fire"],
"threshold":5,"traits":[],"multiattack":2,
"actions":[{"name":"Claw","kind":"melee","toHit":8,"reachFt":10,"damage":"2d8","damageBonus":5,"damageType":"slashing"},
{"name":"Bog Breath","kind":"save","saveAbility":"constitution","dc":15,"damage":"6d6","damageType":"poison","recharge":5}],
"legendary":{"uses":3,"resistance":2,"actions":[{"name":"Tail Sweep","cost":1,"text":"One Claw attack."}]},
"lair":{"actions":[{"name":"Rising Water","text":"The water rises a foot."}],"regional":[]},
"phases":[{"name":"Drowned King","hp":60,"text":"It rises from the water."}]}`

// An author builds the Bog King in the monster builder and reads its stat block and estimated Challenge.
func TestTheMonsterBuilder(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	king := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"creature","name":"Bog King","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/monsters/" + king
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["lines"].([]any)[0] != "Medium Beast" || fresh["estimate"] == "" {
		t.Fatalf("a fresh creature = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/monsters/preview", "dm", `{"name":"Bog King","design":`+bogKingDesign+`}`)
	preview := decode(t, rec)
	if lines := strings.Join(toStrings(preview["lines"].([]any)), "\n"); rec.Code != http.StatusOK || preview["estimate"] != "11" || !strings.Contains(lines, "Lair Actions (initiative 20). Rising Water.") {
		t.Fatalf("preview: %d %v %s", rec.Code, preview["estimate"], lines)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/monsters/preview", "dm", `{"name":"X","design":`+strings.Replace(bogKingDesign, `"ac":16`, `"ac":40`, 1)+`}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an AC of 40: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, "/api/v1/builders/monsters/"+npc, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an NPC in the monster builder: %d", rec.Code)
	}
	saved := decode(t, call(h, http.MethodPut, path, "dm", bogKingDesign))
	if saved["entry"].(map[string]any)["revision"] != float64(2) || !strings.HasPrefix(saved["slug"].(string), "hb-") {
		t.Fatalf("save = %v", saved)
	}
}
