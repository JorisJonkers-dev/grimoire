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

type brokenJournal struct{ err error }

func (b brokenJournal) Read(context.Context, caller.Caller, domain.CampaignID) (campaignapp.JournalView, error) {
	return campaignapp.JournalView{}, b.err
}

func (b brokenJournal) CreateQuest(context.Context, caller.Caller, domain.CampaignID, campaignapp.QuestInput) (domain.Quest, error) {
	return domain.Quest{}, b.err
}

func (b brokenJournal) UpdateQuest(context.Context, caller.Caller, domain.CampaignID, domain.QuestID, campaignapp.QuestInput) (domain.Quest, error) {
	return domain.Quest{}, b.err
}

func (b brokenJournal) DeleteQuest(context.Context, caller.Caller, domain.CampaignID, domain.QuestID) error {
	return b.err
}

func (b brokenJournal) CreateLore(context.Context, caller.Caller, domain.CampaignID, campaignapp.LoreInput) (domain.Lore, error) {
	return domain.Lore{}, b.err
}

func (b brokenJournal) UpdateLore(context.Context, caller.Caller, domain.CampaignID, domain.LoreID, campaignapp.LoreInput) (domain.Lore, error) {
	return domain.Lore{}, b.err
}

func (b brokenJournal) DeleteLore(context.Context, caller.Caller, domain.CampaignID, domain.LoreID) error {
	return b.err
}

func (b brokenJournal) ReadItem(context.Context, caller.Caller, domain.CampaignID, string) (int, error) {
	return 0, b.err
}

// A fault in the Journal service is a fault, and nobody signed out gets anywhere.
func TestJournalErrors(t *testing.T) {
	t.Parallel()
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	quest, lore := c+"/quests/0190c7a8-0000-7000-8000-000000000002", c+"/lore/0190c7a8-0000-7000-8000-000000000003"
	h := campaignServer(t, brokenCampaigns{}, httpapi.JournalService(brokenJournal{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, c + "/journal", ""},
		{http.MethodPost, c + "/journal/readings", `{"itemSlug":"black-book"}`},
		{http.MethodPost, c + "/quests", `{"name":"A","status":"active"}`},
		{http.MethodPut, quest, `{"name":"A","status":"active"}`},
		{http.MethodDelete, quest, ""},
		{http.MethodPost, c + "/lore", `{"title":"A"}`},
		{http.MethodPut, lore, `{"title":"A"}`},
		{http.MethodDelete, lore, ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetJournal(ctx, oas.GetJournalParams{}))
	add(hh.ReadItem(ctx, &oas.ItemReading{}, oas.ReadItemParams{}))
	add(hh.CreateQuest(ctx, &oas.QuestInput{}, oas.CreateQuestParams{}))
	add(hh.UpdateQuest(ctx, &oas.QuestInput{}, oas.UpdateQuestParams{}))
	add(hh.DeleteQuest(ctx, oas.DeleteQuestParams{}))
	add(hh.CreateLore(ctx, &oas.LoreInput{}, oas.CreateLoreParams{}))
	add(hh.UpdateLore(ctx, &oas.LoreInput{}, oas.UpdateLoreParams{}))
	add(hh.DeleteLore(ctx, oas.DeleteLoreParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}

// The DM keeps Quests and Lore over HTTP. A Player's Journal carries neither a hidden Quest nor a
// locked Lore entry, and reading a book from the Party Stash unlocks what it holds for the party.
func TestTheJournalOverHTTP(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	journal := func(who string) (map[string]any, string) {
		t.Helper()
		rec := call(h, http.MethodGet, base+"/journal", who, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s reads the Journal: %d %s", who, rec.Code, rec.Body.String())
		}
		return decode(t, rec), rec.Body.String()
	}

	rec := call(h, http.MethodPost, base+"/quests", "dm", `{"name":"The stolen seal","summary":"The Watch has lost its seal.","status":"active","steps":[{"text":"Ask at the Lantern Inn","done":true},{"text":"Find the fence","done":false}]}`)
	seal := decode(t, rec)
	sealID, _ := seal["id"].(string)
	if steps, _ := seal["steps"].([]any); rec.Code != http.StatusCreated || seal["status"] != "active" || len(steps) != 2 || steps[0].(map[string]any)["done"] != true {
		t.Fatalf("create quest: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base+"/quests", "dm", `{"name":"The traitor in the Watch","status":"hidden"}`)
	traitorID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create a hidden quest: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base+"/lore", "dm", `{"title":"The Ashen Hand","body":"A cult older than the town.","itemSlug":"black-book"}`)
	hand := decode(t, rec)
	handID, _ := hand["id"].(string)
	if rec.Code != http.StatusCreated || hand["unlocked"] != false || hand["itemSlug"] != "black-book" || hand["unlockedAt"] != nil {
		t.Fatalf("create lore: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/lore", "dm", `{"title":"Oakford","body":"A market town.","unlocked":true}`); rec.Code != http.StatusCreated || decode(t, rec)["unlocked"] != true {
		t.Fatalf("create unlocked lore: %d %s", rec.Code, rec.Body.String())
	}

	all, kept := journal("dm")
	if quests, _ := all["quests"].([]any); all["dm"] != true || len(quests) != 2 || len(all["lore"].([]any)) != 2 ||
		!strings.Contains(kept, "The Watch has lost its seal.") || !strings.Contains(kept, "A cult older than the town.") {
		t.Fatalf("the DM's Journal = %s", kept)
	}
	seen, raw := journal("player")
	if quests, _ := seen["quests"].([]any); seen["dm"] != false || len(quests) != 1 || len(seen["lore"].([]any)) != 1 || len(seen["readable"].([]any)) != 0 {
		t.Fatalf("a Player's Journal = %s", raw)
	}
	for _, hidden := range []string{"traitor", "Ashen", "cult", "black-book", "itemSlug", traitorID, handID} {
		if strings.Contains(raw, hidden) {
			t.Fatalf("%q reached a Player: %s", hidden, raw)
		}
	}

	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"signed out":                  {"", http.MethodGet, base + "/journal", "", http.StatusUnauthorized},
		"signed out reads an item":    {"", http.MethodPost, base + "/journal/readings", `{"itemSlug":"black-book"}`, http.StatusUnauthorized},
		"signed out adds a quest":     {"", http.MethodPost, base + "/quests", `{"name":"A","status":"active"}`, http.StatusUnauthorized},
		"signed out changes a quest":  {"", http.MethodPut, base + "/quests/" + sealID, `{"name":"A","status":"active"}`, http.StatusUnauthorized},
		"signed out removes a quest":  {"", http.MethodDelete, base + "/quests/" + sealID, "", http.StatusUnauthorized},
		"signed out adds lore":        {"", http.MethodPost, base + "/lore", `{"title":"A"}`, http.StatusUnauthorized},
		"signed out changes lore":     {"", http.MethodPut, base + "/lore/" + handID, `{"title":"A"}`, http.StatusUnauthorized},
		"signed out removes lore":     {"", http.MethodDelete, base + "/lore/" + handID, "", http.StatusUnauthorized},
		"a stranger reads":            {"stranger", http.MethodGet, base + "/journal", "", http.StatusNotFound},
		"a stranger reads an item":    {"stranger", http.MethodPost, base + "/journal/readings", `{"itemSlug":"black-book"}`, http.StatusNotFound},
		"a player adds a quest":       {"player", http.MethodPost, base + "/quests", `{"name":"Mine","status":"active"}`, http.StatusForbidden},
		"a player changes a quest":    {"player", http.MethodPut, base + "/quests/" + traitorID, `{"name":"Mine","status":"active"}`, http.StatusForbidden},
		"a player removes a quest":    {"player", http.MethodDelete, base + "/quests/" + sealID, "", http.StatusForbidden},
		"a player adds lore":          {"player", http.MethodPost, base + "/lore", `{"title":"Mine","unlocked":true}`, http.StatusForbidden},
		"a player unlocks lore":       {"player", http.MethodPut, base + "/lore/" + handID, `{"title":"The Ashen Hand","unlocked":true}`, http.StatusForbidden},
		"a player removes lore":       {"player", http.MethodDelete, base + "/lore/" + handID, "", http.StatusForbidden},
		"a quest of no status":        {"dm", http.MethodPost, base + "/quests", `{"name":"A","status":"paused"}`, http.StatusBadRequest},
		"a quest with a blank name":   {"dm", http.MethodPost, base + "/quests", `{"name":" ","status":"active"}`, http.StatusUnprocessableEntity},
		"lore with a blank title":     {"dm", http.MethodPost, base + "/lore", `{"title":" "}`, http.StatusUnprocessableEntity},
		"a change to no quest":        {"dm", http.MethodPut, base + "/quests/" + uuid.NewString(), `{"name":"A","status":"active"}`, http.StatusNotFound},
		"the removal of no quest":     {"dm", http.MethodDelete, base + "/quests/" + uuid.NewString(), "", http.StatusNotFound},
		"a change to no lore":         {"dm", http.MethodPut, base + "/lore/" + uuid.NewString(), `{"title":"A"}`, http.StatusNotFound},
		"the removal of no lore":      {"dm", http.MethodDelete, base + "/lore/" + uuid.NewString(), "", http.StatusNotFound},
		"a book nobody carries":       {"player", http.MethodPost, base + "/journal/readings", `{"itemSlug":"black-book"}`, http.StatusUnprocessableEntity},
		"a reading of nothing at all": {"player", http.MethodPost, base + "/journal/readings", `{"itemSlug":""}`, http.StatusBadRequest},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	// An Access Token that may only read cannot keep the Journal, nor read an item, which changes it.
	if rec := withScopes(h, http.MethodPost, base+"/quests", "dm", "read", `{"name":"A","status":"active"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a read token adds a quest: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, base+"/journal/readings", "player", "read build", `{"itemSlug":"black-book"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot play reads an item: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodGet, base+"/journal", "player", "read", ""); rec.Code != http.StatusOK {
		t.Errorf("a read token reads the Journal: %d", rec.Code)
	}

	// The book lands in the Party Stash: the Player's Journal says it can be read, and reading unlocks it.
	stashItems(t, pool, id, map[string]int{"black-book": 1})
	if seen, raw := journal("player"); len(seen["readable"].([]any)) != 1 || seen["readable"].([]any)[0] != "black-book" || strings.Contains(raw, "Ashen") {
		t.Fatalf("with the book in the Stash = %s", raw)
	}
	rec = call(h, http.MethodPost, base+"/journal/readings", "player", `{"itemSlug":"black-book"}`)
	if rec.Code != http.StatusOK || decode(t, rec)["unlocked"] != float64(1) {
		t.Fatalf("reading the book: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/journal/readings", "player", `{"itemSlug":"black-book"}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "nothing to read") {
		t.Fatalf("reading it twice: %d %s", rec.Code, rec.Body.String())
	}
	seen, raw = journal("player")
	lore, _ := seen["lore"].([]any)
	if len(lore) != 2 || lore[1].(map[string]any)["title"] != "The Ashen Hand" || lore[1].(map[string]any)["unlocked"] != true || lore[1].(map[string]any)["unlockedAt"] == nil ||
		strings.Contains(raw, "itemSlug") || len(seen["readable"].([]any)) != 0 {
		t.Fatalf("after reading = %s", raw)
	}

	// The DM gives the hidden Quest, locks the Lore again, and removes the first Quest.
	if rec := call(h, http.MethodPut, base+"/quests/"+traitorID, "dm", `{"name":"The traitor in the Watch","status":"active","steps":[{"text":"Confront Vane","done":false}]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("update quest: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, base+"/lore/"+handID, "dm", `{"title":"The Ashen Hand","body":"A cult older than the town.","itemSlug":"black-book"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("update lore: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, base+"/quests/"+sealID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete quest: %d %s", rec.Code, rec.Body.String())
	}
	seen, raw = journal("player")
	quests, _ := seen["quests"].([]any)
	if len(quests) != 1 || quests[0].(map[string]any)["name"] != "The traitor in the Watch" || len(quests[0].(map[string]any)["steps"].([]any)) != 1 || strings.Contains(raw, "Ashen") {
		t.Fatalf("after the DM's changes = %s", raw)
	}
	if rec := call(h, http.MethodDelete, base+"/lore/"+handID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete lore: %d %s", rec.Code, rec.Body.String())
	}
	if all, raw := journal("dm"); len(all["lore"].([]any)) != 1 || !strings.Contains(raw, `"itemSlug":""`) {
		t.Fatalf("the DM's Journal at the end = %s", raw)
	}
}
