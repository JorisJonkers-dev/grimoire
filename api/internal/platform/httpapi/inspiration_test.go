package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func roster(t *testing.T, h http.Handler, campaign, name string) any {
	t.Helper()
	for _, c := range decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/characters", "player", "")) {
		if c["name"] == name {
			return c["heroicInspiration"]
		}
	}
	return nil
}

// rollD20 asks for a d20 the player throws, and rolls it.
func rollD20(t *testing.T, h http.Handler, rolls string) (string, map[string]any) {
	t.Helper()
	id, _ := decode(t, call(h, http.MethodPost, rolls, "player", `{"purpose":"Athletics","notation":"1d20"}`))["id"].(string)
	return rolls + "/" + id, decode(t, call(h, http.MethodPost, rolls+"/"+id+"/rest", "player", ""))
}

// The DM grants Heroic Inspiration; it shows on the sheet and in the roster. A roll by its holder waits
// to be kept or rerolled once, and a reroll spends it. It can also be passed to an ally that lacks it.
func TestHeroicInspiration(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	chars := "/api/v1/campaigns/" + id + "/characters"
	rolls := "/api/v1/campaigns/" + id + "/rolls"
	kara := chars + "/" + decode(t, call(h, http.MethodPost, chars, "player", build))["id"].(string)
	inesID := decode(t, call(h, http.MethodPost, chars, "dm", strings.Replace(build, `"Kara"`, `"Ines"`, 1)))["id"].(string)

	if _, plain := rollD20(t, h, rolls); plain["status"] != "resolved" || plain["choosing"] != false {
		t.Fatalf("a roll without Inspiration = %v", plain)
	}
	if rec := call(h, http.MethodPatch, kara, "player", `{"heroicInspiration":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player granting it: %d", rec.Code)
	}
	if sheet := decode(t, call(h, http.MethodPatch, kara, "dm", `{"heroicInspiration":true}`)); sheet["heroicInspiration"] != true {
		t.Fatalf("granted = %v", sheet["heroicInspiration"])
	}
	if roster(t, h, id, "Kara") != true || roster(t, h, id, "Ines") != false {
		t.Fatal("the roster does not show Heroic Inspiration")
	}

	one, held := rollD20(t, h, rolls)
	if held["status"] != "pending" || held["choosing"] != true || held["total"] != nil {
		t.Fatalf("a roll with Inspiration = %v", held)
	}
	if rec := call(h, http.MethodPost, one+"/reroll", "dm", `{"die":0}`); rec.Code != http.StatusForbidden {
		t.Fatalf("the DM spending a player's Inspiration: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, one+"/reroll", "player", `{"die":3}`); rec.Code != http.StatusNotFound {
		t.Fatalf("rerolling a die that is not there: %d", rec.Code)
	}
	rerolled := decode(t, call(h, http.MethodPost, one+"/reroll", "player", `{"die":0}`))
	if rerolled["status"] != "resolved" || rerolled["rerolled"] != true || rerolled["choosing"] != false {
		t.Fatalf("rerolled = %v", rerolled)
	}
	if rec := call(h, http.MethodPost, one+"/keep", "player", ""); rec.Code != http.StatusConflict {
		t.Fatalf("keeping a resolved roll: %d", rec.Code)
	}
	if roster(t, h, id, "Kara") != false {
		t.Fatal("a reroll did not spend Heroic Inspiration")
	}
	log := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/log?limit=10", "dm", "").Body.String()
	if strings.Count(log, `"kind":"die_rolled"`) < 3 {
		t.Fatalf("the reroll is not in the Action Log: %s", log)
	}

	call(h, http.MethodPatch, kara, "dm", `{"heroicInspiration":true}`)
	one, _ = rollD20(t, h, rolls)
	if kept := decode(t, call(h, http.MethodPost, one+"/keep", "player", "")); kept["status"] != "resolved" || kept["rerolled"] != false || roster(t, h, id, "Kara") != true {
		t.Fatalf("kept = %v", kept)
	}

	for body, want := range map[string]int{
		`{"to":"` + strings.TrimPrefix(kara, chars+"/") + `"}`: http.StatusUnprocessableEntity,
		`{"to":"0190c7a8-0000-7000-8000-0000000000ff"}`:        http.StatusNotFound,
	} {
		if rec := call(h, http.MethodPost, kara+"/inspiration/pass", "player", body); rec.Code != want {
			t.Fatalf("pass %s: %d", body, rec.Code)
		}
	}
	if rec := call(h, http.MethodPost, kara+"/inspiration/pass", "dm", `{"to":"`+inesID+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("the DM passing a player's Inspiration: %d", rec.Code)
	}
	if sheet := decode(t, call(h, http.MethodPost, kara+"/inspiration/pass", "player", `{"to":"`+inesID+`"}`)); sheet["heroicInspiration"] != false || roster(t, h, id, "Ines") != true {
		t.Fatalf("passed = %v", sheet["heroicInspiration"])
	}
	if rec := call(h, http.MethodPost, kara+"/inspiration/pass", "player", `{"to":"`+inesID+`"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("passing what it no longer has: %d", rec.Code)
	}
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %d: %v", rec.Code, err)
	}
	return out
}
