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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

const gullJSON = `{"name":"The Gull","kind":"water","hullMax":300,"threshold":15,"milesPerDay":48,
"components":[{"name":"Mainsail","hpMax":100,"drives":true},{"name":"Helm","hpMax":50}],
"stations":[{"name":"Helm","crew":1},{"name":"Rigging","crew":6}]}`

// The DM builds a ship, holes it, mends it and crews it over HTTP; every Member sees how it stands and
// how fast it goes.
func TestVehiclesOverHTTP(t *testing.T) {
	t.Parallel()
	h, _ := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	if rec := call(h, http.MethodGet, base+"/vehicles", "player", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"dm":false,"vehicles":[]}` {
		t.Fatalf("no vehicles yet: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(h, http.MethodPost, base+"/vehicles", "dm", gullJSON)
	gull := decode(t, rec)
	gullID, _ := gull["id"].(string)
	parts, _ := gull["components"].([]any)
	posts, _ := gull["stations"].([]any)
	if rec.Code != http.StatusCreated || gull["name"] != "The Gull" || gull["kind"] != "water" || gull["hull"] != float64(300) || gull["hullMax"] != float64(300) || gull["threshold"] != float64(15) ||
		gull["milesPerDay"] != float64(48) || gull["speed"] != float64(24) || gull["shortHanded"] != true || len(parts) != 2 || len(posts) != 2 {
		t.Fatalf("build: %d %s", rec.Code, rec.Body.String())
	}
	sail, helmPart := parts[0].(map[string]any), parts[1].(map[string]any)
	helm, rigging := posts[0].(map[string]any), posts[1].(map[string]any)
	if sail["name"] != "Mainsail" || sail["hp"] != float64(100) || sail["hpMax"] != float64(100) || sail["drives"] != true || helmPart["drives"] != false ||
		helm["name"] != "Helm" || helm["crew"] != float64(1) || helm["posted"] != float64(0) || rigging["crew"] != float64(6) {
		t.Fatalf("its parts: %s", rec.Body.String())
	}
	one := base + "/vehicles/" + gullID
	sailID, helmID, riggingID := sail["id"].(string), helm["id"].(string), rigging["id"].(string)
	// A bare cart needs no threshold, components or stations.
	if rec := call(h, http.MethodPost, base+"/vehicles", "dm", `{"name":"Cart","kind":"land","hullMax":20,"milesPerDay":24}`); rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"threshold":0`) || !strings.Contains(rec.Body.String(), `"speed":24,"shortHanded":false,"components":[],"stations":[]`) {
		t.Fatalf("a cart: %d %s", rec.Code, rec.Body.String())
	}

	for _, post := range [][2]string{{helmID, "1"}, {riggingID, "6"}} {
		rec = call(h, http.MethodPut, one+"/stations/"+post[0], "dm", `{"posted":`+post[1]+`}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("post crew: %d %s", rec.Code, rec.Body.String())
		}
	}
	if got := decode(t, rec); got["speed"] != float64(48) || got["shortHanded"] != false || got["stations"].([]any)[1].(map[string]any)["posted"] != float64(6) {
		t.Fatalf("fully crewed: %s", rec.Body.String())
	}
	rec = call(h, http.MethodPost, one+"/damage", "dm", `{"amount":40}`)
	if got := decode(t, rec); rec.Code != http.StatusOK || got["hull"] != float64(260) || got["speed"] != float64(48) {
		t.Fatalf("a blow to the hull: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, one+"/damage", "dm", `{"amount":100,"componentId":"`+sailID+`"}`)
	if got := decode(t, rec); rec.Code != http.StatusOK || got["hull"] != float64(260) || got["speed"] != float64(0) || got["components"].([]any)[0].(map[string]any)["hp"] != float64(0) {
		t.Fatalf("the sail gone: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, one+"/damage", "dm", `{"amount":30,"componentId":"`+sailID+`","repair":true}`)
	if got := decode(t, rec); rec.Code != http.StatusOK || got["speed"] != float64(48) || got["components"].([]any)[0].(map[string]any)["hp"] != float64(30) {
		t.Fatalf("the sail patched: %d %s", rec.Code, rec.Body.String())
	}
	// A Player sees it all as it stands.
	rec = call(h, http.MethodGet, base+"/vehicles", "player", "")
	list := decode(t, rec)
	if all, _ := list["vehicles"].([]any); rec.Code != http.StatusOK || list["dm"] != false || len(all) != 2 || all[0].(map[string]any)["hull"] != float64(260) || all[0].(map[string]any)["id"] != gullID || all[1].(map[string]any)["name"] != "Cart" {
		t.Fatalf("as a Player sees them: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/vehicles", "dm", ""); !strings.Contains(rec.Body.String(), `"dm":true`) {
		t.Fatalf("as the DM sees them: %s", rec.Body.String())
	}

	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"signed out reads":        {"", http.MethodGet, base + "/vehicles", "", http.StatusUnauthorized},
		"signed out builds":       {"", http.MethodPost, base + "/vehicles", gullJSON, http.StatusUnauthorized},
		"signed out removes":      {"", http.MethodDelete, one, "", http.StatusUnauthorized},
		"signed out strikes":      {"", http.MethodPost, one + "/damage", `{"amount":1}`, http.StatusUnauthorized},
		"signed out posts crew":   {"", http.MethodPut, one + "/stations/" + helmID, `{"posted":1}`, http.StatusUnauthorized},
		"a stranger reads":        {"stranger", http.MethodGet, base + "/vehicles", "", http.StatusNotFound},
		"a player builds":         {"player", http.MethodPost, base + "/vehicles", gullJSON, http.StatusForbidden},
		"a player removes":        {"player", http.MethodDelete, one, "", http.StatusForbidden},
		"a player strikes":        {"player", http.MethodPost, one + "/damage", `{"amount":1}`, http.StatusForbidden},
		"a player posts crew":     {"player", http.MethodPut, one + "/stations/" + helmID, `{"posted":0}`, http.StatusForbidden},
		"no such kind":            {"dm", http.MethodPost, base + "/vehicles", `{"name":"A","kind":"rail","hullMax":1,"milesPerDay":1}`, http.StatusBadRequest},
		"no name":                 {"dm", http.MethodPost, base + "/vehicles", `{"name":" ","kind":"land","hullMax":1,"milesPerDay":1}`, http.StatusUnprocessableEntity},
		"a component named twice": {"dm", http.MethodPost, base + "/vehicles", `{"name":"A","kind":"land","hullMax":1,"milesPerDay":1,"components":[{"name":"Wheel","hpMax":5},{"name":"Wheel","hpMax":5}]}`, http.StatusUnprocessableEntity},
		"no damage":               {"dm", http.MethodPost, one + "/damage", `{"amount":0}`, http.StatusBadRequest},
		"too many crew":           {"dm", http.MethodPut, one + "/stations/" + helmID, `{"posted":2}`, http.StatusUnprocessableEntity},
		"no such vehicle struck":  {"dm", http.MethodPost, base + "/vehicles/" + uuid.NewString() + "/damage", `{"amount":1}`, http.StatusNotFound},
		"no such component":       {"dm", http.MethodPost, one + "/damage", `{"amount":1,"componentId":"` + uuid.NewString() + `"}`, http.StatusNotFound},
		"no such station":         {"dm", http.MethodPut, one + "/stations/" + uuid.NewString(), `{"posted":0}`, http.StatusNotFound},
		"the removal of none":     {"dm", http.MethodDelete, base + "/vehicles/" + uuid.NewString(), "", http.StatusNotFound},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPut, one+"/stations/"+helmID, "dm", `{"posted":2}`); !strings.Contains(rec.Body.String(), "post no more crew than the station takes") {
		t.Errorf("the refusal says why: %s", rec.Body.String())
	}
	// An Access Token reads with read, builds with build, and damages and crews with play.
	for name, c := range map[string]struct {
		method, path, scopes, body string
		want                       int
	}{
		"read reads":                   {http.MethodGet, base + "/vehicles", "read", "", http.StatusOK},
		"play does not read":           {http.MethodGet, base + "/vehicles", "play build", "", http.StatusForbidden},
		"read and play do not build":   {http.MethodPost, base + "/vehicles", "read play", gullJSON, http.StatusForbidden},
		"read and play do not remove":  {http.MethodDelete, one, "read play", "", http.StatusForbidden},
		"read and build do not strike": {http.MethodPost, one + "/damage", "read build", `{"amount":1}`, http.StatusForbidden},
		"read and build do not crew":   {http.MethodPut, one + "/stations/" + helmID, "read build", `{"posted":1}`, http.StatusForbidden},
		"play strikes":                 {http.MethodPost, one + "/damage", "play", `{"amount":1}`, http.StatusOK},
		"play crews":                   {http.MethodPut, one + "/stations/" + helmID, "play", `{"posted":1}`, http.StatusOK},
	} {
		if rec := withScopes(h, c.method, c.path, "dm", c.scopes, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := withScopes(h, http.MethodDelete, one, "dm", "build", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode(t, call(h, http.MethodGet, base+"/vehicles", "dm", "")); len(got["vehicles"].([]any)) != 1 {
		t.Fatalf("at the end = %v", got)
	}
}

type brokenVehicles struct{ err error }

func (b brokenVehicles) List(context.Context, caller.Caller, uuid.UUID) (playapp.VehiclesView, error) {
	return playapp.VehiclesView{}, b.err
}

func (b brokenVehicles) Add(context.Context, caller.Caller, uuid.UUID, vehicles.Design) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

func (b brokenVehicles) Remove(context.Context, caller.Caller, uuid.UUID, uuid.UUID) error {
	return b.err
}

func (b brokenVehicles) Strike(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.VehicleBlow) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

func (b brokenVehicles) Post(context.Context, caller.Caller, uuid.UUID, uuid.UUID, uuid.UUID, int) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

// A fault in the vehicle service is a fault, and nobody signed out gets anywhere.
func TestVehicleErrors(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/vehicles"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.VehicleService(brokenVehicles{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"name":"A","kind":"land","hullMax":1,"milesPerDay":1}`},
		{http.MethodDelete, one, ""},
		{http.MethodPost, one + "/damage", `{"amount":1}`},
		{http.MethodPut, one + "/stations/0190c7a8-0000-7000-8000-000000000003", `{"posted":1}`},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListVehicles(ctx, oas.ListVehiclesParams{}))
	add(hh.CreateVehicle(ctx, &oas.VehicleInput{}, oas.CreateVehicleParams{}))
	add(hh.DeleteVehicle(ctx, oas.DeleteVehicleParams{}))
	add(hh.DamageVehicle(ctx, &oas.VehicleBlow{}, oas.DamageVehicleParams{}))
	add(hh.PostVehicleCrew(ctx, &oas.VehicleCrew{}, oas.PostVehicleCrewParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
