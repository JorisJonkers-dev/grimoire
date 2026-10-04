package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mcpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// mcpStack is the API with MCP over a real database, and a compendium that knows goblins and rope.
func mcpStack(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	goblin := snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", XP: 50, Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves: map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{}, Senses: map[string]int{}, Resistances: []string{}, Immunities: []string{},
		Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{}, Actions: []snapshot.Action{},
	}
	compendium := comppg.New(store.Pool())
	if _, err := compendium.Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{goblin},
		Items:     []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope", Description: "Rope."}, Category: "gear", CostGP: 1, WeightLB: 5}},
	}, "mcp"); err != nil {
		t.Fatal(err)
	}
	repo := campaignpg.New(store.Pool())
	lib := &libraryapp.Service{
		Repo: librarypg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now, Log: quiet,
		Admins: admins{}, Surfaces: playpg.New(store.Pool()).SurfaceKinds,
	}
	h, err := httpapi.New(httpapi.Options{
		Handler: &httpapi.Handler{
			Version: "1", Store: fakeStore{}, Compendium: compendium, Campaigns: app.NewService(repo), NPCs: &app.NPCs{Repo: repo, Now: time.Now}, Log: quiet,
			Factions: &app.Factions{Repo: repo, Now: time.Now},
			Library:  lib, RollTableBuilds: lib,
			Prep: &prepapp.Service{
				Repo: preppg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now,
				Seed: func() uint64 { return 3 }, Source: func(seed uint64) dice.Source { return rng.New(seed) },
			},
		},
		RateLimit: 1000, Now: time.Now, Edits: repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, store.Pool()
}

type as string

func (a as) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-User-Id", string(a))
	return http.DefaultTransport.RoundTrip(r)
}

// agent is an MCP client signed in as one account.
type agent struct {
	t *testing.T
	s *mcp.ClientSession
}

func newAgent(t *testing.T, url, subject string) agent {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "prep-agent", Version: "1"}, nil)
	s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url + "/mcp", HTTPClient: &http.Client{Transport: as(subject)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return agent{t: t, s: s}
}

type answer struct {
	Result   json.RawMessage `json:"result"`
	Revision struct {
		ID     string `json:"id"`
		No     int    `json:"no"`
		Action string `json:"action"`
	} `json:"revision"`
}

// call runs a tool and returns its answer, or the refusal it gave.
func (a agent) call(tool string, args map[string]any) (answer, string) {
	a.t.Helper()
	res, err := a.s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		a.t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		return answer{}, text
	}
	var out answer
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		a.t.Fatalf("%s: %s", tool, text)
	}
	return out, ""
}

// must runs a tool that has to succeed.
func (a agent) must(tool string, args map[string]any) answer {
	a.t.Helper()
	out, refused := a.call(tool, args)
	if refused != "" {
		a.t.Fatalf("%s refused: %s", tool, refused)
	}
	return out
}

func idOf(t *testing.T, a answer) string {
	t.Helper()
	var x struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(a.Result, &x); err != nil || x.ID == "" {
		t.Fatalf("no id in %s", a.Result)
	}
	return x.ID
}

func TestAnAgentPrepsACampaignThroughMCPAndTheDMUndoesIt(t *testing.T) {
	t.Parallel()
	h, pool := mcpStack(t)
	id, _ := campaignWithPlayer(t, h)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dm, player, stranger := newAgent(t, srv.URL, "dm"), newAgent(t, srv.URL, "player"), newAgent(t, srv.URL, "stranger")
	tools, err := dm.s.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != len(mcpapi.Tools()) {
		t.Fatalf("tools: %v", err)
	}
	cid := map[string]any{"campaignId": id}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{"campaignId": id}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	made := dm.must("create_npc", with(map[string]any{"body": map[string]any{"name": "Tamsin", "title": "Innkeeper", "disposition": "friendly"}}))
	npc := idOf(t, made)
	if made.Revision.No != 1 || made.Revision.Action != "create" {
		t.Fatalf("create revision %+v", made.Revision)
	}
	edited := dm.must("update_npc", with(map[string]any{"npcId": npc, "body": map[string]any{"name": "Tamsin", "title": "Spy", "disposition": "hostile"}}))
	if edited.Revision.No != 2 || edited.Revision.Action != "update" {
		t.Fatalf("update revision %+v", edited.Revision)
	}
	feed := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/activity", "dm", "")
	var activity []map[string]any
	if err := json.Unmarshal(feed.Body.Bytes(), &activity); err != nil || len(activity) != 2 {
		t.Fatalf("activity: %s", feed.Body.String())
	}
	if activity[0]["client"] != "prep-agent" || activity[0]["origin"] != "mcp" || activity[0]["name"] != "Tamsin" || activity[0]["undoable"] != true || activity[1]["undoable"] != false {
		t.Fatalf("activity: %s", feed.Body.String())
	}
	if out := dm.must("list_activity", cid); !strings.Contains(string(out.Result), edited.Revision.ID) {
		t.Fatalf("list_activity: %s", out.Result)
	}
	if _, refused := dm.call("undo_change", with(map[string]any{"revisionId": made.Revision.ID})); refused != "Undo the later changes to Tamsin first." {
		t.Fatalf("undo an older change: %q", refused)
	}
	undone := dm.must("undo_change", with(map[string]any{"revisionId": edited.Revision.ID}))
	if !strings.Contains(string(undone.Result), `"action":"restore"`) || !strings.Contains(string(undone.Result), `"no":3`) {
		t.Fatalf("undo: %s", undone.Result)
	}
	if got := dm.must("get_npc", with(map[string]any{"npcId": npc})); !strings.Contains(string(got.Result), `"title":"Innkeeper"`) {
		t.Fatalf("after undo: %s", got.Result)
	}
	gone := dm.must("delete_npc", with(map[string]any{"npcId": npc}))
	if gone.Revision.Action != "delete" || string(gone.Result) != "null" {
		t.Fatalf("delete: %+v", gone)
	}
	undo := func(rev string) map[string]any {
		t.Helper()
		rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/activity/"+rev+"/undo", "dm", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("undo %s: %d %s", rev, rec.Code, rec.Body.String())
		}
		return decode(t, rec)
	}
	back := undo(gone.Revision.ID)
	if _, refused := dm.call("get_npc", with(map[string]any{"npcId": npc})); refused != "" || back["origin"] != "ui" {
		t.Fatalf("undo a delete: %v %q", back, refused)
	}
	undo(back["revisionId"].(string))
	if _, refused := dm.call("get_npc", with(map[string]any{"npcId": npc})); refused == "" {
		t.Fatal("undoing the restore of a deleted NPC deletes it again")
	}

	town := idOf(t, dm.must("create_settlement", with(map[string]any{"body": map[string]any{"name": "Oakford", "size": "town", "wealth": "modest"}})))
	shopBody := map[string]any{"settlementId": town, "name": "Store", "kind": "general", "markupPct": 0, "haggleDc": 12, "hagglePct": 10, "restock": "never"}
	pool1 := map[string]any{"name": "Band", "levelMin": 1, "levelMax": 4, "difficulty": "low", "members": []any{map[string]any{"monsterSlug": "goblin", "weight": 1, "min": 1, "max": 3}}}
	table := map[string]any{"name": "Road", "chancePct": 20, "visibility": "open", "entries": []any{map[string]any{"weight": 1, "kind": "nothing", "label": ""}}}
	loot := map[string]any{"name": "Purse", "rolls": 1, "entries": []any{map[string]any{"weight": 1, "kind": "nothing"}}}
	for _, c := range []struct {
		kind, idParam string
		body          map[string]any
		rename        string
	}{
		{"shop", "shopId", shopBody, "Smithy"},
		{"encounter_pool", "poolId", pool1, "Warband"},
		{"encounter_table", "tableId", table, "Trail"},
		{"loot_table", "lootTableId", loot, "Hoard"},
		{"settlement", "settlementId", map[string]any{"name": "Mill", "size": "hamlet", "wealth": "poor"}, "Weir"},
	} {
		first := dm.must("create_"+c.kind, with(map[string]any{"body": c.body}))
		thing := idOf(t, first)
		renamed := map[string]any{}
		for k, v := range c.body {
			renamed[k] = v
		}
		renamed["name"] = c.rename
		second := dm.must("update_"+c.kind, with(map[string]any{c.idParam: thing, "body": renamed}))
		if second.Revision.No != 2 {
			t.Fatalf("%s: %+v", c.kind, second.Revision)
		}
		if back := undo(second.Revision.ID); back["name"] != c.body["name"] {
			t.Fatalf("%s undo update: %v", c.kind, back)
		}
		other := dm.must("create_"+c.kind, with(map[string]any{"body": c.body}))
		if back := undo(other.Revision.ID); back["action"] != "delete" {
			t.Fatalf("%s undo create: %v", c.kind, back)
		}
		listed := dm.must("list_"+c.kind+"s", cid)
		if strings.Contains(string(listed.Result), idOf(t, other)) || !strings.Contains(string(listed.Result), thing) {
			t.Fatalf("%s list: %s", c.kind, listed.Result)
		}
	}
	shop := dm.must("list_shops", cid)
	var shops []struct{ ID string }
	_ = json.Unmarshal(shop.Result, &shops)
	if out := dm.must("reroll_stock", with(map[string]any{"shopId": shops[0].ID})); out.Revision.Action != "update" {
		t.Fatalf("reroll: %+v", out.Revision)
	}

	var check string
	if err := pool.QueryRow(context.Background(), `INSERT INTO campaign.revisions (campaign_id, entity_type, entity_id, revision_no, action, caller_subject, caller_name, origin)
		VALUES ($1, 'encounter_check', $2, 1, 'create', 'dm', 'Joris', 'system') RETURNING id`, id, uuid.New()).Scan(&check); err != nil {
		t.Fatal(err)
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/activity/"+check+"/undo", "dm", ""); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "This change cannot be undone.") {
		t.Fatalf("undo a check: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/activity/"+uuid.NewString()+"/undo", "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("undo nothing: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/activity", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player activity: %d", rec.Code)
	}

	for _, who := range []agent{player, stranger} {
		for _, tool := range []string{"list_npcs", "create_npc", "get_campaign"} {
			if _, refused := who.call(tool, with(map[string]any{"body": map[string]any{"name": "X", "disposition": "neutral"}})); refused == "" {
				t.Fatalf("%s ran for a non-DM", tool)
			}
		}
	}
	if _, refused := player.call("list_npcs", cid); refused != "Only the campaign's DM can use Grimoire's tools on it." {
		t.Fatalf("player refusal: %q", refused)
	}
	if _, refused := dm.call("create_npc", with(map[string]any{"body": map[string]any{"name": ""}})); refused == "" {
		t.Fatal("an invalid NPC was created")
	}
	if found := player.must("search_compendium", map[string]any{"kind": "monster", "q": "gob"}); !strings.Contains(string(found.Result), "goblin") {
		t.Fatalf("search: %s", found.Result)
	}
	if rec := call(h, http.MethodPost, "/mcp", "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}
