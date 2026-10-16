//go:build integration

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// A DM adds Factions from the catalogue, suggests Standing Changes and decides them. An Access Token
// may suggest and never decide. A Player's answer carries tiers and shared reasons, and no number.
func TestFactionsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/factions"

	rec := call(h, http.MethodGet, "/api/v1/faction-archetypes", "player", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"slug":"city-watch"`) || strings.Count(rec.Body.String(), `"slug"`) != 11 {
		t.Fatalf("archetypes: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/faction-archetypes", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("archetypes signed out: %d", rec.Code)
	}
	rec = call(h, http.MethodPost, base, "dm", `{"name":"The Lantern Watch","archetype":"city-watch","goals":"Keep the peace.","territory":"Oakford","notes":"The captain takes bribes."}`)
	watch := decode(t, rec)
	watchID, _ := watch["id"].(string)
	secrets, _ := watch["dm"].(map[string]any)
	if rec.Code != http.StatusCreated || watch["tier"] != "neutral" || watch["archetype"] != "city-watch" || secrets["score"] != float64(0) || secrets["notes"] != "The captain takes bribes." {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"a player creates":      {"player", http.MethodPost, base, `{"name":"Mine"}`, http.StatusForbidden},
		"a stranger lists":      {"stranger", http.MethodGet, base, "", http.StatusNotFound},
		"an unknown archetype":  {"dm", http.MethodPost, base, `{"name":"Harpers","archetype":"harpers"}`, http.StatusUnprocessableEntity},
		"a player suggests":     {"player", http.MethodPost, base + "/" + watchID + "/standing-changes", `{"delta":50,"reason":"We are great."}`, http.StatusForbidden},
		"a change of nothing":   {"dm", http.MethodPost, base + "/" + watchID + "/standing-changes", `{"delta":0,"reason":"Nothing."}`, http.StatusUnprocessableEntity},
		"a change for no one":   {"dm", http.MethodPost, base + "/" + uuid.NewString() + "/standing-changes", `{"delta":5,"reason":"Where?"}`, http.StatusNotFound},
		"an update of no one":   {"dm", http.MethodPut, base + "/" + uuid.NewString(), `{"name":"Nobody"}`, http.StatusNotFound},
		"a player removes":      {"player", http.MethodDelete, base + "/" + watchID, "", http.StatusForbidden},
		"a decision on nothing": {"dm", http.MethodPost, "/api/v1/campaigns/" + id + "/standing-changes/" + uuid.NewString() + "/decision", `{"confirm":true}`, http.StatusNotFound},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}

	// An Access Token that may build suggests a change; it waits, and the token cannot decide it.
	changes := base + "/" + watchID + "/standing-changes"
	rec = withScopes(h, http.MethodPost, changes, "dm", "read build play", `{"delta":-70,"reason":"Killed a watchman."}`)
	killed := decode(t, rec)
	killedID, _ := killed["id"].(string)
	if rec.Code != http.StatusCreated || killed["status"] != "pending" || killed["rose"] != false {
		t.Fatalf("a token suggests: %d %s", rec.Code, rec.Body.String())
	}
	decide := func(change string) string {
		return "/api/v1/campaigns/" + id + "/standing-changes/" + change + "/decision"
	}
	if rec := withScopes(h, http.MethodPost, decide(killedID), "dm", "read build play", `{"confirm":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a token decides: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, decide(killedID), "player", `{"confirm":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player decides: %d", rec.Code)
	}
	list := call(h, http.MethodGet, base, "dm", "")
	if !strings.Contains(list.Body.String(), `"tier":"neutral"`) || !strings.Contains(list.Body.String(), `"status":"pending"`) || !strings.Contains(list.Body.String(), `"delta":-70`) {
		t.Fatalf("the DM's list while it waits: %s", list.Body.String())
	}
	if seen := call(h, http.MethodGet, base, "player", ""); strings.Contains(seen.Body.String(), "watchman") || strings.Contains(seen.Body.String(), "pending") {
		t.Fatalf("a pending change reached a Player: %s", seen.Body.String())
	}

	// The DM, signed in, confirms it at a smaller amount and keeps the reason; a second one is shared.
	if rec := call(h, http.MethodPost, decide(killedID), "dm", `{"confirm":true,"delta":-30,"shareReason":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, decide(killedID), "dm", `{"confirm":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("confirm twice: %d", rec.Code)
	}
	seal := decode(t, call(h, http.MethodPost, changes, "dm", `{"delta":5,"reason":"Returned the stolen seal.","shareReason":true}`))
	sealID, _ := seal["id"].(string)
	if rec := call(h, http.MethodPost, decide(sealID), "dm", `{"confirm":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("confirm the second: %d", rec.Code)
	}
	gone := decode(t, call(h, http.MethodPost, changes, "dm", `{"delta":40,"reason":"A rumour.","shareReason":true}`))
	goneID, _ := gone["id"].(string)
	if rec := call(h, http.MethodPost, decide(goneID), "dm", `{"confirm":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("dismiss: %d", rec.Code)
	}

	// The Player's answer: the tier and the shared reason; no score, amount, unshared reason, DM part,
	// dismissed change or note.
	seen := call(h, http.MethodGet, base, "player", "")
	body := seen.Body.String()
	if seen.Code != 200 || !strings.Contains(body, `"tier":"unfriendly"`) || !strings.Contains(body, "Returned the stolen seal.") || strings.Count(body, `"status":"confirmed"`) != 2 {
		t.Fatalf("a Player's list: %d %s", seen.Code, body)
	}
	for _, secret := range []string{"watchman", "bribes", "Keep the peace", "Oakford", "rumour", `"dm"`, "score", "delta", ":-25", ":-30", "shareReason", "origin", "dismissed"} {
		if strings.Contains(body, secret) {
			t.Errorf("%q reached a Player: %s", secret, body)
		}
	}
	if dm := call(h, http.MethodGet, base, "dm", "").Body.String(); !strings.Contains(dm, `"score":-25`) || !strings.Contains(dm, `"status":"dismissed"`) || !strings.Contains(dm, "Killed a watchman.") {
		t.Fatalf("the DM's list: %s", dm)
	}
	if rec := call(h, http.MethodPut, base+"/"+watchID, "dm", `{"name":"The Watch"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, base+"/"+watchID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base, "dm", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("after removing: %s", rec.Body.String())
	}
}
