package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// noSession is a Campaign with no Session under way to tell of thresholds crossed.
type noSession struct{}

func (noSession) TrackCrossed(domain.CampaignID, domain.Member, caller.Caller, []campaignapp.TrackCrossing) {
}

// The DM keeps Tracks and moves their scores over HTTP; a Player's answer carries their own
// Character's score and the party's, and no threshold.
func TestTracksOverHTTP(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/tracks"
	// A Roll Table of the DM's Library that the Campaign sees.
	madness := uuid.NewString()
	for _, q := range []string{
		`INSERT INTO library.entries (id, owner_subject, kind, name, design, created_at, updated_at) VALUES ($1, 'dm', 'table', 'Madness', '{"dice":"1d4","results":[{"from":1,"to":4,"text":"x"}]}', now(), now())`,
		`INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES ('` + id + `', $1, now(), now())`,
	} {
		if _, err := pool.Exec(context.Background(), q, madness); err != nil {
			t.Fatal(err)
		}
	}
	kara := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", "player", srdFighter))["id"].(string)
	ines := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", "dm", srdScholar))["id"].(string)

	rec := call(h, http.MethodPost, base, "dm", `{"name":"Stress","scope":"character","min":0,"max":10,"start":1,"thresholds":[
		{"at":4,"rising":true,"label":"Shaken","effect":"frightened"},{"at":2,"rising":false,"label":"Calm again"},
		{"at":8,"rising":true,"label":"Breaking point","rollTableId":"`+madness+`"}]}`)
	stress := decode(t, rec)
	stressID, _ := stress["id"].(string)
	thresholds, _ := stress["thresholds"].([]any)
	if rec.Code != http.StatusCreated || stress["scope"] != "character" || stress["start"] != float64(1) || len(thresholds) != 3 || thresholds[0].(map[string]any)["effect"] != "frightened" || thresholds[1].(map[string]any)["effect"] != nil ||
		thresholds[2].(map[string]any)["rollTableId"] != madness || thresholds[0].(map[string]any)["rollTableId"] != nil {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base, "dm", `{"name":"Renown","scope":"party","min":-5,"max":5,"start":0}`)
	renownID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create a party Track: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h, http.MethodPost, base+"/"+stressID+"/adjustments", "dm", `{"characterId":"`+kara+`","delta":4}`)
	moved := decode(t, rec)
	if crossed, _ := moved["crossed"].([]any); rec.Code != http.StatusOK || moved["value"] != float64(5) || len(crossed) != 1 || crossed[0].(map[string]any)["label"] != "Shaken" || crossed[0].(map[string]any)["effect"] != "frightened" {
		t.Fatalf("adjust: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/"+renownID+"/adjustments", "dm", `{"delta":-2}`); rec.Code != http.StatusOK || decode(t, rec)["value"] != float64(-2) {
		t.Fatalf("adjust the party's: %d %s", rec.Code, rec.Body.String())
	}

	all := decode(t, call(h, http.MethodGet, base, "dm", ""))
	tracks, _ := all["tracks"].([]any)
	first, _ := tracks[0].(map[string]any)
	if standings, _ := first["standings"].([]any); all["dm"] != true || len(tracks) != 2 || len(first["thresholds"].([]any)) != 3 || first["thresholds"].([]any)[2].(map[string]any)["rollTableId"] != madness || len(standings) != 2 {
		t.Fatalf("the DM's list = %v", all)
	}
	rec = call(h, http.MethodGet, base, "player", "")
	seen := decode(t, rec)
	mine, _ := seen["tracks"].([]any)[0].(map[string]any)
	standings, _ := mine["standings"].([]any)
	party, _ := seen["tracks"].([]any)[1].(map[string]any)["standings"].([]any)
	if seen["dm"] != false || mine["thresholds"] != nil || len(standings) != 1 || standings[0].(map[string]any)["characterId"] != kara || standings[0].(map[string]any)["value"] != float64(5) ||
		len(party) != 1 || party[0].(map[string]any)["value"] != float64(-2) || party[0].(map[string]any)["characterId"] != nil {
		t.Fatalf("a Player's list = %s", rec.Body.String())
	}
	for _, hidden := range []string{"Shaken", "frightened", "Calm again", "thresholds", madness, ines} {
		if strings.Contains(rec.Body.String(), hidden) {
			t.Fatalf("%q reached a Player: %s", hidden, rec.Body.String())
		}
	}

	adjust := base + "/" + stressID + "/adjustments"
	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"signed out lists":            {"", http.MethodGet, base, "", http.StatusUnauthorized},
		"signed out creates":          {"", http.MethodPost, base, `{"name":"A","scope":"party","min":0,"max":1,"start":0}`, http.StatusUnauthorized},
		"signed out removes":          {"", http.MethodDelete, base + "/" + stressID, "", http.StatusUnauthorized},
		"signed out adjusts":          {"", http.MethodPost, adjust, `{"delta":1}`, http.StatusUnauthorized},
		"a stranger lists":            {"stranger", http.MethodGet, base, "", http.StatusNotFound},
		"a player creates":            {"player", http.MethodPost, base, `{"name":"Mine","scope":"party","min":0,"max":1,"start":0}`, http.StatusForbidden},
		"a player removes":            {"player", http.MethodDelete, base + "/" + stressID, "", http.StatusForbidden},
		"a player adjusts":            {"player", http.MethodPost, adjust, `{"characterId":"` + kara + `","delta":-5}`, http.StatusForbidden},
		"no such scope":               {"dm", http.MethodPost, base, `{"name":"A","scope":"faction","min":0,"max":1,"start":0}`, http.StatusBadRequest},
		"bounds the wrong way":        {"dm", http.MethodPost, base, `{"name":"A","scope":"party","min":5,"max":1,"start":3}`, http.StatusUnprocessableEntity},
		"a threshold of two":          {"dm", http.MethodPost, base, `{"name":"A","scope":"party","min":0,"max":9,"start":0,"thresholds":[{"at":3,"rising":true,"label":"x","effect":"prone","rollTableId":"` + uuid.NewString() + `"}]}`, http.StatusUnprocessableEntity},
		"a move of nothing":           {"dm", http.MethodPost, adjust, `{"characterId":"` + kara + `","delta":0}`, http.StatusUnprocessableEntity},
		"a Character's for the party": {"dm", http.MethodPost, adjust, `{"delta":1}`, http.StatusUnprocessableEntity},
		"no such Character":           {"dm", http.MethodPost, adjust, `{"characterId":"` + uuid.NewString() + `","delta":1}`, http.StatusNotFound},
		"no such Track":               {"dm", http.MethodPost, base + "/" + uuid.NewString() + "/adjustments", `{"delta":1}`, http.StatusNotFound},
		"the removal of none":         {"dm", http.MethodDelete, base + "/" + uuid.NewString(), "", http.StatusNotFound},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	// An Access Token moves a score only when it may play, and builds a Track only when it may build.
	if rec := withScopes(h, http.MethodPost, adjust, "dm", "read build", `{"characterId":"`+kara+`","delta":1}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot play adjusts: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, base, "dm", "read play", `{"name":"A","scope":"party","min":0,"max":1,"start":0}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot build creates: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, adjust, "dm", "read play", `{"characterId":"`+kara+`","delta":-4}`); rec.Code != http.StatusOK || decode(t, rec)["value"] != float64(1) {
		t.Errorf("a token that may play adjusts: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, base+"/"+stressID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if after := decode(t, call(h, http.MethodGet, base, "dm", "")); len(after["tracks"].([]any)) != 1 {
		t.Fatalf("after removing = %v", after)
	}
}

type brokenTracks struct{ err error }

func (b brokenTracks) List(context.Context, caller.Caller, domain.CampaignID) (campaignapp.TracksView, error) {
	return campaignapp.TracksView{}, b.err
}

func (b brokenTracks) Create(context.Context, caller.Caller, domain.CampaignID, campaignapp.TrackInput) (domain.Track, error) {
	return domain.Track{}, b.err
}

func (b brokenTracks) Delete(context.Context, caller.Caller, domain.CampaignID, domain.TrackID) error {
	return b.err
}

func (b brokenTracks) Adjust(context.Context, caller.Caller, domain.CampaignID, domain.TrackID, *domain.CharacterID, int) (campaignapp.TrackAdjusted, error) {
	return campaignapp.TrackAdjusted{}, b.err
}

// A fault in the Track service is a fault, and nobody signed out gets anywhere.
func TestTrackErrors(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/tracks"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.TrackService(brokenTracks{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"name":"A","scope":"party","min":0,"max":1,"start":0}`},
		{http.MethodDelete, one, ""},
		{http.MethodPost, one + "/adjustments", `{"delta":1}`},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListTracks(ctx, oas.ListTracksParams{}))
	add(hh.CreateTrack(ctx, &oas.TrackInput{}, oas.CreateTrackParams{}))
	add(hh.DeleteTrack(ctx, oas.DeleteTrackParams{}))
	add(hh.AdjustTrack(ctx, &oas.TrackAdjustment{}, oas.AdjustTrackParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
