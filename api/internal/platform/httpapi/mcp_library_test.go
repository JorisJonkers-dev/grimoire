package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

// scopedAs is an MCP client signed in as one account through an Access Token of some scopes.
type scopedAs struct{ subject, scopes string }

func (w scopedAs) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-User-Id", w.subject)
	r.Header.Set(httpx.ScopesHeader, w.scopes)
	return http.DefaultTransport.RoundTrip(r)
}

func tokenAgent(t *testing.T, url, subject, scopes string) agent {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "scoped-agent", Version: "1"}, nil)
	s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url + "/mcp", HTTPClient: &http.Client{Transport: scopedAs{subject: subject, scopes: scopes}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return agent{t: t, s: s}
}

// kept is the part of a tool's answer that says how its write is kept.
type kept struct {
	LibraryRevision struct {
		EntryID string `json:"entryId"`
		No      int    `json:"no"`
		Origin  string `json:"origin"`
		Client  string `json:"client"`
	} `json:"libraryRevision"`
	Pending bool `json:"pending"`
}

// said runs a tool and reads how its write is kept from the answer.
func (a agent) said(tool string, args map[string]any) (kept, json.RawMessage) {
	a.t.Helper()
	res, err := a.s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		a.t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if res.IsError {
		a.t.Fatalf("%s refused: %s", tool, text)
	}
	var out struct {
		kept
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		a.t.Fatalf("%s: %s", tool, text)
	}
	return out.kept, out.Result
}

// A DM's agent builds Homebrew in the DM's Library through MCP: every write is a Revision that says an
// agent made it and which, and the DM brings an earlier one back. A Standing Change an agent suggests
// waits for the DM. Nobody's agent reaches another's Library, or a Campaign its account does not run.
func TestAnAgentBuildsHomebrewAndSuggestsStandingThroughMCP(t *testing.T) {
	t.Parallel()
	h, pool := mcpStack(t)
	id, _ := campaignWithPlayer(t, h)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dm, player := newAgent(t, srv.URL, "dm"), newAgent(t, srv.URL, "player")

	// A creature in the DM's Library, made and then edited by the agent.
	made, result := dm.said("create_library_entry", map[string]any{"body": map[string]any{"kind": "creature", "name": "Bog Hag", "fields": []any{map[string]any{"name": "HP", "value": "52"}}}})
	var entry struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result, &entry); err != nil || entry.ID == "" {
		t.Fatalf("create: %s", result)
	}
	if rev := made.LibraryRevision; rev.EntryID != entry.ID || rev.No != 1 || rev.Origin != "mcp" || rev.Client != "prep-agent" || made.Pending {
		t.Fatalf("the Revision a create makes = %+v", made)
	}
	edited, _ := dm.said("update_library_entry", map[string]any{"entryId": entry.ID, "body": map[string]any{"name": "Bog Hag Matriarch", "fields": []any{map[string]any{"name": "HP", "value": "80"}}}})
	if rev := edited.LibraryRevision; rev.No != 2 || rev.Origin != "mcp" || rev.Client != "prep-agent" {
		t.Fatalf("the Revision an edit makes = %+v", edited)
	}
	// The DM reads who made what in the app, and brings the first Revision back by hand.
	detail := decode(t, call(h, http.MethodGet, "/api/v1/library/"+entry.ID, "dm", ""))
	revs, _ := detail["revisions"].([]any)
	if newest, _ := revs[0].(map[string]any); len(revs) != 2 || newest["origin"] != "mcp" || newest["client"] != "prep-agent" {
		t.Fatalf("the Revisions as the DM reads them = %v", revs)
	}
	if rec := call(h, http.MethodPost, "/api/v1/library/"+entry.ID+"/revisions/1/restore", "dm", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Bog Hag"`) {
		t.Fatalf("the DM restores: %d %s", rec.Code, rec.Body.String())
	}
	byHand := decode(t, call(h, http.MethodGet, "/api/v1/library/"+entry.ID, "dm", ""))["revisions"].([]any)[0].(map[string]any)
	if byHand["no"] != float64(3) || byHand["origin"] != "ui" || byHand["client"] != nil {
		t.Fatalf("the Revision the DM made by hand = %v", byHand)
	}
	// The agent can bring one back too, and reads the entry with its Revisions.
	back, _ := dm.said("restore_library_revision", map[string]any{"entryId": entry.ID, "revisionNo": 2})
	if rev := back.LibraryRevision; rev.No != 4 || rev.Origin != "mcp" {
		t.Fatalf("the Revision a restore makes = %+v", back)
	}
	got := dm.must("get_library_entry", map[string]any{"entryId": entry.ID})
	if !strings.Contains(string(got.Result), `"name":"Bog Hag Matriarch"`) || strings.Count(string(got.Result), `"origin"`) != 4 {
		t.Fatalf("the entry as the agent reads it = %s", got.Result)
	}
	if list := dm.must("list_library_entries", map[string]any{"kind": "creature"}); !strings.Contains(string(list.Result), entry.ID) {
		t.Fatalf("the Library as the agent lists it = %s", list.Result)
	}

	// A Roll Table designed through its builder: a design the rules refuse is refused with the reason.
	table := idOf(t, dm.must("create_library_entry", map[string]any{"body": map[string]any{"kind": "table", "name": "Fumbles", "fields": []any{}}}))
	var design map[string]any
	if err := json.Unmarshal([]byte(fumbleDesign), &design); err != nil {
		t.Fatal(err)
	}
	saved, _ := dm.said("save_roll_table_build", map[string]any{"entryId": table, "body": design})
	if rev := saved.LibraryRevision; rev.EntryID != table || rev.No != 2 || rev.Origin != "mcp" {
		t.Fatalf("the Revision a design makes = %+v", saved)
	}
	if build := dm.must("get_roll_table_build", map[string]any{"entryId": table}); !strings.Contains(string(build.Result), "You fall flat on your face.") {
		t.Fatalf("the design as the agent reads it = %s", build.Result)
	}
	if _, refused := dm.call("save_roll_table_build", map[string]any{"entryId": table, "body": map[string]any{"dice": "1d6", "results": []any{}}}); refused == "" {
		t.Fatal("an empty Roll Table was saved")
	}

	// Another account's agent finds none of it and changes none of it.
	if list := player.must("list_library_entries", nil); string(list.Result) != "[]" {
		t.Fatalf("the Player's agent lists the DM's Library: %s", list.Result)
	}
	for tool, args := range map[string]map[string]any{
		"get_library_entry":        {"entryId": entry.ID},
		"update_library_entry":     {"entryId": entry.ID, "body": map[string]any{"name": "Mine", "fields": []any{}}},
		"restore_library_revision": {"entryId": entry.ID, "revisionNo": 1},
		"save_roll_table_build":    {"entryId": table, "body": design},
		"get_roll_table_build":     {"entryId": table},
	} {
		if _, refused := player.call(tool, args); refused == "" {
			t.Errorf("the Player's agent used %s on the DM's entry", tool)
		}
	}
	var revisions int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM library.entry_revisions WHERE entry_id = $1", uuid.MustParse(entry.ID)).Scan(&revisions); err != nil || revisions != 4 {
		t.Fatalf("the entry's Revisions after all that was refused: %d %v", revisions, err)
	}

	// A Standing Change the agent suggests waits for the DM, and says an agent suggested it.
	watch := decode(t, call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/factions", "dm", `{"name":"The Lantern Watch"}`))["id"].(string)
	suggested, change := dm.said("propose_standing_change", map[string]any{"campaignId": id, "factionId": watch, "body": map[string]any{"delta": 3, "reason": "They saved the mill."}})
	if !suggested.Pending || !strings.Contains(string(change), `"status":"pending"`) || !strings.Contains(string(change), `"client":"prep-agent"`) {
		t.Fatalf("a suggestion = %+v %s", suggested, change)
	}
	factions := dm.must("list_factions", map[string]any{"campaignId": id})
	if !strings.Contains(string(factions.Result), `"tier":"neutral"`) || !strings.Contains(string(factions.Result), `"score":0`) {
		t.Fatalf("the Faction has not moved = %s", factions.Result)
	}
	// Tools on a Campaign are for its DM's agent alone.
	for tool, args := range map[string]map[string]any{
		"propose_standing_change": {"campaignId": id, "factionId": watch, "body": map[string]any{"delta": 3, "reason": "We are great."}},
		"list_factions":           {"campaignId": id},
		"list_linked_entries":     {"campaignId": id},
	} {
		if _, refused := player.call(tool, args); refused != "Only the campaign's DM can use Grimoire's tools on it." {
			t.Errorf("the Player's agent on %s: %q", tool, refused)
		}
	}
	if linked := dm.must("list_linked_entries", map[string]any{"campaignId": id}); string(linked.Result) != "[]" {
		t.Fatalf("linked entries = %s", linked.Result)
	}

	// An Access Token's scopes hold through MCP: reading is not building.
	reader := tokenAgent(t, srv.URL, "dm", "read")
	if list := reader.must("list_library_entries", nil); !strings.Contains(string(list.Result), entry.ID) {
		t.Fatalf("a read token lists: %s", list.Result)
	}
	for tool, args := range map[string]map[string]any{
		"create_library_entry":     {"body": map[string]any{"kind": "npc", "name": "Odo", "fields": []any{}}},
		"update_library_entry":     {"entryId": entry.ID, "body": map[string]any{"name": "Odo", "fields": []any{}}},
		"restore_library_revision": {"entryId": entry.ID, "revisionNo": 1},
		"save_roll_table_build":    {"entryId": table, "body": design},
		"propose_standing_change":  {"campaignId": id, "factionId": watch, "body": map[string]any{"delta": 1, "reason": "x"}},
	} {
		if _, refused := reader.call(tool, args); refused == "" {
			t.Errorf("a read token used %s", tool)
		}
	}
	builder := tokenAgent(t, srv.URL, "dm", "read build")
	if made, _ := builder.said("update_library_entry", map[string]any{"entryId": entry.ID, "body": map[string]any{"name": "Bog Hag", "fields": []any{}}}); made.LibraryRevision.No != 5 || made.LibraryRevision.Client != "scoped-agent" {
		t.Fatalf("a build token edits = %+v", made)
	}
}
