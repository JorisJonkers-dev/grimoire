package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// The DM gives downtime and keeps Recipes over HTTP; a Player crafts with their own Character's days,
// and the Game Clock moves on.
func TestDowntimeOverHTTP(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	kara := decode(t, call(h, http.MethodPost, base+"/characters", "player", srdFighter))["id"].(string)
	ines := decode(t, call(h, http.MethodPost, base+"/characters", "dm", srdScholar))["id"].(string)
	// Kara's Inventory exists once it is opened; she is given the makings and the coin.
	if rec := call(h, http.MethodGet, base+"/characters/"+kara+"/inventory", "player", ""); rec.Code != http.StatusOK {
		t.Fatalf("inventory: %d %s", rec.Code, rec.Body.String())
	}
	for _, q := range []string{
		`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
			SELECT gen_random_uuid(), id, 'healing-herb', 3, true, false, now() FROM campaign.containers WHERE character_id = $1`,
		`DELETE FROM campaign.container_coins WHERE container_id IN (SELECT id FROM campaign.containers WHERE character_id = $1)`,
		`INSERT INTO campaign.container_coins (container_id, coin, amount) SELECT id, 'gp', 30 FROM campaign.containers WHERE character_id = $1`,
	} {
		if _, err := pool.Exec(context.Background(), q, kara); err != nil {
			t.Fatal(err)
		}
	}

	rec := call(h, http.MethodPost, base+"/recipes", "dm", `{"name":"Healing salve","makes":"healing-salve","quantity":2,"tool":"","days":3,"costCp":2500,"ingredients":[{"item":"healing-herb","count":2}]}`)
	salve := decode(t, rec)
	salveID, _ := salve["id"].(string)
	if ing, _ := salve["ingredients"].([]any); rec.Code != http.StatusCreated || salve["makes"] != "healing-salve" || salve["costCp"] != float64(2500) || salve["tool"] != nil || len(ing) != 1 || ing[0].(map[string]any)["count"] != float64(2) {
		t.Fatalf("create recipe: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base+"/recipes", "dm", `{"name":"Whittling","makes":"wooden-spoon","quantity":1,"tool":"carving-knife","days":1}`)
	spoonID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"tool":"carving-knife"`) {
		t.Fatalf("create a second recipe: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, base+"/downtime/grants", "dm", `{"days":5}`)
	before := decode(t, rec)
	if chars, _ := before["characters"].([]any); rec.Code != http.StatusOK || before["dm"] != true || len(chars) != 2 || chars[0].(map[string]any)["days"] != float64(5) || chars[0].(map[string]any)["mine"] != true {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/downtime/grants", "dm", `{"days":2,"characterId":"`+ines+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("grant to one: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, base+"/characters/"+kara+"/downtime", "player", `{"activity":"craft","recipeId":"`+salveID+`"}`)
	after := decode(t, rec)
	if rec.Code != http.StatusOK || after["dm"] != false || after["gameDay"] != before["gameDay"].(float64)+3 || len(after["recipes"].([]any)) != 2 {
		t.Fatalf("craft: %d %s", rec.Code, rec.Body.String())
	}
	days := map[string]float64{}
	mine := map[string]bool{}
	for _, c := range after["characters"].([]any) {
		row, _ := c.(map[string]any)
		days[row["id"].(string)], mine[row["id"].(string)] = row["days"].(float64), row["mine"].(bool)
	}
	if days[kara] != 2 || days[ines] != 7 || !mine[kara] || mine[ines] {
		t.Fatalf("after crafting: days %v, mine %v", days, mine)
	}
	if log, _ := after["log"].([]any); len(log) != 1 || log[0].(map[string]any)["activity"] != "craft" || log[0].(map[string]any)["detail"] != "Healing salve" || log[0].(map[string]any)["days"] != float64(3) || log[0].(map[string]any)["character"] == "" {
		t.Fatalf("the log = %v", after["log"])
	}
	inv := call(h, http.MethodGet, base+"/characters/"+kara+"/inventory", "player", "").Body.String()
	if !strings.Contains(inv, "healing-salve") {
		t.Fatalf("her Inventory after crafting: %s", inv)
	}
	// Two days of work use up the two days she has left; the DM gives her one more.
	rec = call(h, http.MethodPost, base+"/characters/"+kara+"/downtime", "player", `{"activity":"work","days":2}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+kara+`","name":"Kara","days":0`) {
		t.Fatalf("work: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/downtime/grants", "dm", `{"days":1,"characterId":"`+kara+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("one more day: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/characters/"+kara+"/downtime", "player", `{"activity":"research","days":1,"subject":"The Ashen Hand"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "The Ashen Hand") {
		t.Fatalf("research: %d %s", rec.Code, rec.Body.String())
	}

	spend := base + "/characters/" + kara + "/downtime"
	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"signed out reads":          {"", http.MethodGet, base + "/downtime", "", http.StatusUnauthorized},
		"signed out grants":         {"", http.MethodPost, base + "/downtime/grants", `{"days":1}`, http.StatusUnauthorized},
		"signed out spends":         {"", http.MethodPost, spend, `{"activity":"work","days":1}`, http.StatusUnauthorized},
		"signed out adds a recipe":  {"", http.MethodPost, base + "/recipes", `{"name":"A","makes":"a","quantity":1,"days":1}`, http.StatusUnauthorized},
		"signed out removes one":    {"", http.MethodDelete, base + "/recipes/" + salveID, "", http.StatusUnauthorized},
		"a stranger reads":          {"stranger", http.MethodGet, base + "/downtime", "", http.StatusNotFound},
		"a player grants":           {"player", http.MethodPost, base + "/downtime/grants", `{"days":1}`, http.StatusForbidden},
		"a player adds a recipe":    {"player", http.MethodPost, base + "/recipes", `{"name":"A","makes":"a","quantity":1,"days":1}`, http.StatusForbidden},
		"a player removes one":      {"player", http.MethodDelete, base + "/recipes/" + salveID, "", http.StatusForbidden},
		"a player spends another's": {"player", http.MethodPost, base + "/characters/" + ines + "/downtime", `{"activity":"work","days":1}`, http.StatusForbidden},
		"no such activity":          {"player", http.MethodPost, spend, `{"activity":"carouse","days":1}`, http.StatusBadRequest},
		"too many days":             {"player", http.MethodPost, spend, `{"activity":"work","days":9}`, http.StatusUnprocessableEntity},
		"a tool not at hand":        {"player", http.MethodPost, spend, `{"activity":"craft","recipeId":"` + spoonID + `"}`, http.StatusUnprocessableEntity},
		"no such recipe":            {"player", http.MethodPost, spend, `{"activity":"craft","recipeId":"` + uuid.NewString() + `"}`, http.StatusNotFound},
		"a recipe with no name":     {"dm", http.MethodPost, base + "/recipes", `{"name":" ","makes":"a","quantity":1,"days":1}`, http.StatusUnprocessableEntity},
		"a grant to nobody":         {"dm", http.MethodPost, base + "/downtime/grants", `{"days":1,"characterId":"` + uuid.NewString() + `"}`, http.StatusNotFound},
		"the removal of none":       {"dm", http.MethodDelete, base + "/recipes/" + uuid.NewString(), "", http.StatusNotFound},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPost, spend, "player", `{"activity":"craft","recipeId":"`+spoonID+`"}`); !strings.Contains(rec.Body.String(), "It needs a carving-knife at hand.") {
		t.Errorf("the refusal says why: %s", rec.Body.String())
	}
	// An Access Token that may only read sees the downtime and spends none of it.
	if rec := withScopes(h, http.MethodPost, spend, "player", "read build", `{"activity":"work","days":1}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot play spends: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, base+"/recipes", "dm", "read play", `{"name":"A","makes":"a","quantity":1,"days":1}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot build adds a recipe: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodGet, base+"/downtime", "player", "read", ""); rec.Code != http.StatusOK {
		t.Errorf("a read token reads: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, base+"/recipes/"+spoonID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode(t, call(h, http.MethodGet, base+"/downtime", "dm", "")); len(got["recipes"].([]any)) != 1 || len(got["log"].([]any)) != 3 {
		t.Fatalf("at the end = %v", got)
	}
}

type brokenDowntime struct{ err error }

func (b brokenDowntime) View(context.Context, caller.Caller, uuid.UUID) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

func (b brokenDowntime) Grant(context.Context, caller.Caller, uuid.UUID, *uuid.UUID, int) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

func (b brokenDowntime) AddRecipe(context.Context, caller.Caller, uuid.UUID, downtime.Recipe) (playdomain.CampaignRecipe, error) {
	return playdomain.CampaignRecipe{}, b.err
}

func (b brokenDowntime) RemoveRecipe(context.Context, caller.Caller, uuid.UUID, uuid.UUID) error {
	return b.err
}

func (b brokenDowntime) Spend(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.DowntimeActivity) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

// A fault in the downtime service is a fault, and nobody signed out gets anywhere.
func TestDowntimeErrors(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	h := campaignServer(t, brokenCampaigns{}, httpapi.DowntimeService(brokenDowntime{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base + "/downtime", ""},
		{http.MethodPost, base + "/downtime/grants", `{"days":1}`},
		{http.MethodPost, base + "/characters/0190c7a8-0000-7000-8000-000000000002/downtime", `{"activity":"work","days":1}`},
		{http.MethodPost, base + "/recipes", `{"name":"A","makes":"a","quantity":1,"days":1}`},
		{http.MethodDelete, base + "/recipes/0190c7a8-0000-7000-8000-000000000003", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetDowntime(ctx, oas.GetDowntimeParams{}))
	add(hh.GrantDowntime(ctx, &oas.DowntimeGrant{}, oas.GrantDowntimeParams{}))
	add(hh.SpendDowntime(ctx, &oas.DowntimeActivity{}, oas.SpendDowntimeParams{}))
	add(hh.CreateRecipe(ctx, &oas.RecipeInput{}, oas.CreateRecipeParams{}))
	add(hh.DeleteRecipe(ctx, oas.DeleteRecipeParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
