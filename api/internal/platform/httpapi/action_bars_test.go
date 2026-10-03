package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

// A player arranges a Character's action bars once: the layout is kept on the Character, so it is the
// same in every Campaign the Character plays in, and nobody else can read or change it.
func TestACharactersActionBars(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	first, _ := campaignWithPlayer(t, h)
	call(h, http.MethodPost, "/api/v1/campaigns/"+first+"/characters", "player", build)
	items, _ := decode(t, call(h, http.MethodGet, "/api/v1/characters", "player", ""))["items"].([]any)
	id, _ := items[0].(map[string]any)["id"].(string)
	path := "/api/v1/characters/" + id + "/action-bars"

	fresh := decode(t, call(h, http.MethodGet, path, "player", ""))
	if bars, _ := fresh["bars"].([]any); len(bars) != 0 || len(fresh["quick"].([]any)) != 0 || len(fresh["stowed"].([]any)) != 0 || fresh["arranged"] != false {
		t.Fatalf("a Character nobody arranged = %v", fresh)
	}
	layout := `{"bars":[["attack:Longsword","action:dash","spell:fireball"],["unarmed:grapple"]],"quick":["attack:Longsword","action:dash"],"stowed":["action:hide"]}`
	rec := call(h, http.MethodPut, path, "player", layout)
	saved := decode(t, rec)
	if bars, _ := saved["bars"].([]any); rec.Code != http.StatusOK || len(bars) != 2 || len(bars[0].([]any)) != 3 || saved["arranged"] != true {
		t.Fatalf("save: %d %v", rec.Code, saved)
	}
	second := secondCampaign(t, h, "Saltmarsh")
	if rec := call(h, http.MethodPost, "/api/v1/characters/"+id+"/campaigns", "player", `{"campaignId":"`+second+`"}`); rec.Code != http.StatusCreated {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	again := decode(t, call(h, http.MethodGet, path, "player", ""))
	if quick, _ := again["quick"].([]any); len(quick) != 2 || quick[1] != "action:dash" || again["bars"].([]any)[1].([]any)[0] != "unarmed:grapple" || again["stowed"].([]any)[0] != "action:hide" {
		t.Fatalf("the layout follows the Character = %v", again)
	}

	for who, method := range map[string]string{"dm": http.MethodGet, "stranger": http.MethodPut} {
		if rec := call(h, method, path, who, layout); rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s on someone else's bars: %d", who, method, rec.Code)
		}
	}
	eleven := `"action:a","action:b","action:c","action:d","action:e","action:f","action:g","action:h","action:i","action:j","action:k"`
	for name, body := range map[string]string{
		"three bars":             `{"bars":[[],[],[]],"quick":[],"stowed":[]}`,
		"eleven tiles on a bar":  `{"bars":[[` + eleven + `]],"quick":[],"stowed":[]}`,
		"five quick tiles":       `{"bars":[],"quick":["action:a","action:b","action:c","action:d","action:e"],"stowed":[]}`,
		"a tile twice":           `{"bars":[["action:dash"],["action:dash"]],"quick":[],"stowed":[]}`,
		"a quick tile twice":     `{"bars":[],"quick":["action:dash","action:dash"],"stowed":[]}`,
		"a tile of no kind":      `{"bars":[["dash"]],"quick":[],"stowed":[]}`,
		"a tile of unknown kind": `{"bars":[["potion:healing"]],"quick":[],"stowed":[]}`,
		"a nameless tile":        `{"bars":[["attack:"]],"quick":[],"stowed":[]}`,
		"a stowed tile on a bar": `{"bars":[["action:dash"]],"quick":[],"stowed":["action:dash"]}`,
		"a layout with no stow":  `{"bars":[],"quick":[]}`,
		"a quick nameless tile":  `{"bars":[],"quick":["spell: "],"stowed":[]}`,
	} {
		if rec := call(h, http.MethodPut, path, "player", body); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if kept := decode(t, call(h, http.MethodGet, path, "player", "")); !strings.Contains(kept["bars"].([]any)[0].([]any)[2].(string), "fireball") {
		t.Fatalf("a refused layout changed the saved one: %v", kept)
	}
	if rec := call(h, http.MethodPut, path, "player", `{"bars":[],"quick":[],"stowed":[]}`); rec.Code != http.StatusOK || decode(t, rec)["arranged"] != true {
		t.Fatalf("emptying the bars: %d", rec.Code)
	}
}
