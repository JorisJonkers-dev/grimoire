package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
)

// bestiaryOf is the creatures a test Campaign has.
type bestiaryOf []string

func (b bestiaryOf) Creature(_ context.Context, _ domain.CampaignID, slug string) error {
	if slices.Contains(b, slug) {
		return nil
	}
	return domain.ErrNotFound
}

// The DM keeps the allies who travel with the party: a creature of the Campaign under a name of its
// own, run by a Player or by the DM, with or without a share of the XP. Every Member sees them; only
// the DM changes them, and the DM's notes stay the DM's.
func TestCompanionsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	var tamsin string
	members, _ := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+id, "dm", ""))["members"].([]any)
	for _, m := range members {
		if member, _ := m.(map[string]any); member["displayName"] == "Tamsin" {
			tamsin, _ = member["id"].(string)
		}
	}
	base := "/api/v1/campaigns/" + id + "/companions"
	body := func(name, kind, slug, controller string, shares bool, notes string) string {
		who := ""
		if controller != "" {
			who = `,"controllerId":"` + controller + `"`
		}
		share := "false"
		if shares {
			share = "true"
		}
		return `{"name":"` + name + `","kind":"` + kind + `","monsterSlug":"` + slug + `"` + who + `,"sharesXp":` + share + `,"notes":"` + notes + `"}`
	}
	rec := call(h, http.MethodPost, base, "dm", body("  Fang ", "companion", "wolf", tamsin, true, "Bites strangers."))
	fang := decode(t, rec)
	fangID, _ := fang["id"].(string)
	if rec.Code != http.StatusCreated || fang["name"] != "Fang" || fang["kind"] != "companion" || fang["monsterSlug"] != "wolf" || fang["controllerId"] != tamsin ||
		fang["sharesXp"] != true || fang["notes"] != "Bites strangers." || fang["hp"] != nil {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	for name, c := range map[string]struct {
		who, body string
		want      int
	}{
		"a player":                {"player", body("Rex", "companion", "wolf", "", false, ""), http.StatusForbidden},
		"an outsider":             {"stranger", body("Rex", "companion", "wolf", "", false, ""), http.StatusNotFound},
		"a nameless ally":         {"dm", body("  ", "companion", "wolf", "", false, ""), http.StatusUnprocessableEntity},
		"a name too long":         {"dm", body(strings.Repeat("x", 41), "companion", "wolf", "", false, ""), http.StatusUnprocessableEntity},
		"a creature not there":    {"dm", body("Rex", "companion", "dragon", "", false, ""), http.StatusUnprocessableEntity},
		"no creature":             {"dm", body("Rex", "companion", " ", "", false, ""), http.StatusUnprocessableEntity},
		"a kind there is not":     {"dm", body("Rex", "pet", "wolf", "", false, ""), http.StatusBadRequest},
		"someone not in the game": {"dm", body("Rex", "companion", "wolf", "0190c7a8-0000-7000-8000-0000000000ff", false, ""), http.StatusUnprocessableEntity},
	} {
		if rec := call(h, http.MethodPost, base, c.who, c.body); rec.Code != c.want {
			t.Fatalf("%s adds a Companion: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	rec = call(h, http.MethodPost, base, "dm", body("Bors", "hireling", "goblin", "", false, ""))
	if rec.Code != http.StatusCreated || decode(t, rec)["controllerId"] != nil || decode(t, rec)["kind"] != "hireling" {
		t.Fatalf("a hireling the DM runs: %d %s", rec.Code, rec.Body.String())
	}

	// Every Member sees them, by name; the notes are the DM's.
	for who, notes := range map[string]string{"dm": "Bites strangers.", "player": ""} {
		rec := call(h, http.MethodGet, base, who, "")
		var list []map[string]any
		if err := jsonUnmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != http.StatusOK || len(list) != 2 || list[0]["name"] != "Bors" || list[1]["name"] != "Fang" || list[1]["notes"] != notes {
			t.Fatalf("%s lists: %d %s", who, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodGet, base, "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("an outsider lists: %d", rec.Code)
	}

	// The DM takes Fang over, without its share.
	one := base + "/" + fangID
	rec = call(h, http.MethodPut, one, "dm", body("Old Fang", "companion", "wolf", "", false, ""))
	if got := decode(t, rec); rec.Code != http.StatusOK || got["name"] != "Old Fang" || got["controllerId"] != nil || got["sharesXp"] != false || got["id"] != fangID {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	for name, c := range map[string]struct {
		method, path, who string
		want              int
	}{
		"a player changes one":       {http.MethodPut, one, "player", http.StatusForbidden},
		"a player lets one go":       {http.MethodDelete, one, "player", http.StatusForbidden},
		"an outsider lets one go":    {http.MethodDelete, one, "stranger", http.StatusNotFound},
		"changing one that is not":   {http.MethodPut, base + "/0190c7a8-0000-7000-8000-0000000000ff", "dm", http.StatusNotFound},
		"letting go one that is not": {http.MethodDelete, base + "/0190c7a8-0000-7000-8000-0000000000ff", "dm", http.StatusNotFound},
	} {
		payload := ""
		if c.method == http.MethodPut {
			payload = body("Rex", "companion", "wolf", "", false, "")
		}
		if rec := call(h, c.method, c.path, c.who, payload); rec.Code != c.want {
			t.Fatalf("%s: %d, want %d", name, rec.Code, c.want)
		}
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: %d", rec.Code)
	}
}
