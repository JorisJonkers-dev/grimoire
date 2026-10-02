package httpapi_test

import (
	"net/http"
	"testing"
)

// The sheet shows every attack with its bonus, damage and Weapon Mastery, the features and traits the
// Character has, and its training, all worked out by the rules.
func TestSheetShowsAttacksTraitsAndTraining(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	created := decode(t, call(h, http.MethodPost, base, "player", build))
	chID, _ := created["id"].(string)
	if rec := call(h, http.MethodPatch, base+"/"+chID, "player", `{"weapons":["longsword","blowgun"]}`); rec.Code != http.StatusOK {
		t.Fatalf("arm: %d %s", rec.Code, rec.Body.String())
	}
	sheet := decode(t, call(h, http.MethodGet, base+"/"+chID, "player", ""))
	attacks, _ := sheet["attacks"].([]any)
	if len(attacks) != 2 {
		t.Fatalf("attacks = %v", sheet["attacks"])
	}
	sword, _ := attacks[0].(map[string]any)
	if sword["toHit"] != float64(5) || sword["damage"] != "1d8+3" || sword["mastery"] != "sap" || sword["reachFeet"] != float64(5) {
		t.Fatalf("longsword = %v", sword)
	}
	blowgun, _ := attacks[1].(map[string]any)
	if blowgun["toHit"] != float64(4) || blowgun["damage"] != "3" || blowgun["rangeFeet"] != float64(25) || blowgun["reachFeet"] != float64(0) || blowgun["mastery"] != nil {
		t.Fatalf("blowgun = %v", blowgun)
	}
	traits, _ := sheet["traits"].([]any)
	if len(traits) != 1 || traits[0].(map[string]any)["name"] != "Second Wind" {
		t.Fatalf("traits = %v", sheet["traits"])
	}
	training, _ := sheet["proficiencies"].(map[string]any)
	if armor, _ := training["armor"].([]any); len(armor) == 0 {
		t.Fatalf("proficiencies = %v", training)
	}
	if owned, _ := sheet["characterId"].(string); owned == "" || sheet["tempHp"] != float64(0) {
		t.Fatalf("characterId %v, tempHp %v", sheet["characterId"], sheet["tempHp"])
	}
	skills, _ := sheet["skills"].([]any)
	if _, ok := skills[0].(map[string]any)["expertise"].(bool); !ok {
		t.Fatalf("skill = %v", skills[0])
	}
}

// Damage soaks temporary hit points first, healing stops at the maximum, and new temporary hit points
// keep the higher of old and new.
func TestSheetTakesDamageAndHeals(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	created := decode(t, call(h, http.MethodPost, base, "player", build))
	chID, _ := created["id"].(string)
	steps := []struct {
		body     string
		hp, temp float64
	}{
		{`{"tempHp":4}`, 12, 4},
		{`{"damage":6}`, 10, 0},
		{`{"damage":50}`, 0, 0},
		{`{"heal":5}`, 5, 0},
		{`{"heal":50}`, 12, 0},
		{`{"tempHp":3}`, 12, 3},
		{`{"tempHp":2}`, 12, 3},
	}
	for _, s := range steps {
		rec := call(h, http.MethodPatch, base+"/"+chID, "player", s.body)
		got := decode(t, rec)
		if rec.Code != http.StatusOK || got["hpCurrent"] != s.hp || got["tempHp"] != s.temp {
			t.Fatalf("%s: %d hp %v temp %v", s.body, rec.Code, got["hpCurrent"], got["tempHp"])
		}
	}
	if rec := call(h, http.MethodPatch, base+"/"+chID, "player", `{"damage":-1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("negative damage: %d", rec.Code)
	}
}
