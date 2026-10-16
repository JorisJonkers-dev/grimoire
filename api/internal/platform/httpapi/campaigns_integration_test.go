//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func realCampaigns(t *testing.T) http.Handler {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	repo := campaignpg.New(store.Pool())
	parts := []any{
		&app.Characters{
			Repo: repo, Compendium: &fakeCompendium{}, Combat: app.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now,
			Roll: func() []int { return []int{16, 15, 12, 11, 9, 8} },
		},
		&app.NPCs{Repo: repo, Now: time.Now},
		&app.Companions{Repo: repo, Creatures: bestiaryOf{"goblin", "wolf"}, Now: time.Now},
		httpapi.FactionService(&app.Factions{Repo: repo, Now: time.Now}),
		httpapi.RuleVariantService(&app.RuleVariants{Repo: repo, Now: time.Now}),
		httpapi.RollService(&playapp.Rolls{
			Repo: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo},
			Seed: rng.Seed, Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now,
		}),
	}
	return campaignServer(t, app.NewService(repo), append(parts, liveParts(t, store.Pool(), repo)...)...)
}

// campaignWithPlayer returns the id of a campaign run by "dm" that "player" joined.
func campaignWithPlayer(t *testing.T, h http.Handler) (string, map[string]any) {
	t.Helper()
	rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"Morvain","displayName":"Joris"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := decode(t, rec)["id"].(string)
	rec = call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/invites", "dm", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	invite := decode(t, rec)
	token, _ := invite["token"].(string)
	rec = call(h, http.MethodPost, "/api/v1/invites/accept", "player", `{"token":"`+token+`","displayName":"Tamsin"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	return id, invite
}

func TestCampaignLifecycleOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, invite := campaignWithPlayer(t, h)
	token, _ := invite["token"].(string)
	rec := call(h, http.MethodPost, "/api/v1/invites/preview", "someone", `{"token":"`+token+`"}`)
	if rec.Code != 200 || decode(t, rec)["campaignName"] != "Morvain" {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns/"+id, "player", "")
	home := decode(t, rec)
	members, _ := home["members"].([]any)
	if rec.Code != 200 || home["myRole"] != "player" || len(members) != 2 {
		t.Fatalf("home: %d %v", rec.Code, home)
	}
	player, _ := members[1].(map[string]any)
	playerID, _ := player["id"].(string)
	if player["isMe"] != true {
		t.Fatalf("player row = %v", player)
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"name":"Greyfen","ruleset":"srd-2024","reactionTimeoutS":5,"highGround":true,"noUndo":true,"showDcs":true,"restSupplies":true,"initiativeMode":"side","shareInitiative":true}`)
	if body := decode(t, rec); rec.Code != 200 || body["ruleset"] != "srd-2024" || body["reactionTimeoutS"] != float64(5) || body["highGround"] != true || body["noUndo"] != true || body["showDcs"] != true || body["restSupplies"] != true ||
		body["initiativeMode"] != "side" || body["shareInitiative"] != true {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	// A Campaign starts at the rules as written with plain dice; the DM picks a difficulty preset and
	// karmic dice, and every Member can read both.
	if home["difficulty"] != "standard" || home["karmicDice"] != false {
		t.Fatalf("a new Campaign's difficulty and dice = %v %v", home["difficulty"], home["karmicDice"])
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"difficulty":"story","karmicDice":true}`)
	if body := decode(t, rec); rec.Code != 200 || body["difficulty"] != "story" || body["karmicDice"] != true || body["highGround"] != true || body["name"] != "Greyfen" {
		t.Fatalf("difficulty: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"difficulty":"hard"}`)
	if body := decode(t, rec); rec.Code != 200 || body["difficulty"] != "hard" || body["karmicDice"] != true {
		t.Fatalf("difficulty alone: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"karmicDice":false}`)
	if body := decode(t, rec); rec.Code != 200 || body["difficulty"] != "hard" || body["karmicDice"] != false {
		t.Fatalf("dice alone: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"difficulty":"nightmare"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown difficulty: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "player", `{"difficulty":"story"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a Player sets the difficulty: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns/"+id, "player", "")
	if body := decode(t, rec); rec.Code != 200 || body["difficulty"] != "hard" || body["karmicDice"] != false {
		t.Fatalf("a Player reads the difficulty: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns", "player", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"difficulty":"hard"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id+"/members/"+playerID, "dm", `{"role":"dm"}`)
	if rec.Code != 200 || decode(t, rec)["role"] != "dm" {
		t.Fatalf("co-DM: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/invites", "player", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Joris") {
		t.Fatalf("invites: %d %s", rec.Code, rec.Body.String())
	}
	inviteID, _ := invite["id"].(string)
	if rec := call(h, http.MethodDelete, "/api/v1/campaigns/"+id+"/invites/"+inviteID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, "/api/v1/campaigns/"+id+"/members/"+playerID, "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("leave: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id, "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("after leaving: %d", rec.Code)
	}
}

func TestCampaignListPages(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	for _, name := range []string{"One", "Two", "Three"} {
		if rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"`+name+`","displayName":"DM"}`); rec.Code != 201 {
			t.Fatalf("create: %d", rec.Code)
		}
	}
	rec := call(h, http.MethodGet, "/api/v1/campaigns?limit=2", "dm", "")
	page := decode(t, rec)
	items, _ := page["items"].([]any)
	next, _ := page["nextCursor"].(string)
	if rec.Code != 200 || len(items) != 2 || next == "" {
		t.Fatalf("page: %d %v", rec.Code, page)
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns?limit=2&cursor="+next, "dm", "")
	rest, _ := decode(t, rec)["items"].([]any)
	if rec.Code != 200 || len(rest) != 1 {
		t.Fatalf("second page: %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{"AAAA", "e30", "A"} {
		if rec := call(h, http.MethodGet, "/api/v1/campaigns?cursor="+bad, "dm", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("cursor %q: %d", bad, rec.Code)
		}
	}
}

func TestEveryCampaignOperationRefusesOutsiders(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, invite := campaignWithPlayer(t, h)
	inviteID, _ := invite["id"].(string)
	home := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+id, "dm", ""))
	members, _ := home["members"].([]any)
	dmRow, _ := members[0].(map[string]any)
	dmID, _ := dmRow["id"].(string)
	base := "/api/v1/campaigns/" + id
	type op struct{ method, path, body string }
	dmOnly := []op{
		{http.MethodPatch, base, `{"name":"X"}`},
		{http.MethodPatch, base + "/members/" + dmID, `{"role":"player"}`},
		{http.MethodDelete, base + "/members/" + dmID, ""},
		{http.MethodGet, base + "/invites", ""},
		{http.MethodPost, base + "/invites", ""},
		{http.MethodDelete, base + "/invites/" + inviteID, ""},
	}
	for _, o := range append([]op{{http.MethodGet, base, ""}}, dmOnly...) {
		if rec := call(h, o.method, o.path, "stranger", o.body); rec.Code != http.StatusNotFound {
			t.Errorf("stranger %s %s: %d", o.method, o.path, rec.Code)
		}
		if rec := call(h, o.method, o.path, "", o.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", o.method, o.path, rec.Code)
		}
	}
	for _, o := range dmOnly {
		if rec := call(h, o.method, o.path, "player", o.body); rec.Code != http.StatusForbidden {
			t.Errorf("player %s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCampaignEdgeCasesOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	home := decode(t, call(h, http.MethodGet, base, "dm", ""))
	members, _ := home["members"].([]any)
	dmRow, _ := members[0].(map[string]any)
	dmID, _ := dmRow["id"].(string)
	if rec := call(h, http.MethodPatch, base+"/members/"+dmID, "dm", `{"role":"player"}`); rec.Code != http.StatusConflict {
		t.Errorf("last DM stepped down: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"  ","displayName":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("blank name: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"Old rules","displayName":"x","ruleset":"srd-2014"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("2014 ruleset accepted for a new campaign: %d", rec.Code)
	}
	if rec := call(h, http.MethodPatch, base, "dm", `{"ruleset":"srd-2014"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("campaign switched to the 2014 ruleset: %d", rec.Code)
	}
	unknown := `{"token":"` + strings.Repeat("x", 43) + `"}`
	if rec := call(h, http.MethodPost, "/api/v1/invites/preview", "someone", unknown); rec.Code != http.StatusNotFound {
		t.Errorf("unknown invite: %d", rec.Code)
	}
	for _, path := range []string{"/api/v1/campaigns", "/api/v1/invites/preview", "/api/v1/invites/accept"} {
		if rec := call(h, http.MethodPost, path, "", `{}`); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
			t.Errorf("anonymous %s: %d", path, rec.Code)
		}
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous list: %d", rec.Code)
	}
}

func liveParts(t *testing.T, pool *pgxpool.Pool, repo *campaignpg.Store) []any {
	t.Helper()
	members := playpg.CampaignMembers{Store: repo}
	hub := &live.Hub{Store: playpg.New(pool), Members: members, Owner: playpg.Owner{Pool: pool}, Now: time.Now, Log: quiet}
	t.Cleanup(hub.Shutdown)
	sessions := &playapp.Sessions{Repo: playpg.New(pool), Members: members, Live: hub, Now: time.Now}
	maps := &playapp.Maps{Repo: playpg.New(pool), Members: members, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now}
	return []any{httpapi.SessionService(sessions), httpapi.LiveHub(hub), httpapi.LiveMembers(members), httpapi.MapService(maps)}
}
