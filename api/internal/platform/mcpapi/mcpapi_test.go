package mcpapi_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	campaign "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mcpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func TestToolsMatchTheSpecAndItsRoutes(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile("../../../../openapi/v1/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	exported, err := mcpapi.Export(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalise(t, exported), normalise(t, mcpapi.Tools())) {
		t.Fatal("tools.json is stale; run task gen")
	}
	router, err := oas.NewServer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tool := range mcpapi.Tools() {
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %s", tool.Name)
		}
		seen[tool.Name] = true
		path := tool.Path
		for _, p := range tool.PathParams {
			path = strings.Replace(path, "{"+p+"}", "x", 1)
		}
		if _, ok := router.FindRoute(tool.Method, path); !ok {
			t.Fatalf("%s: no route for %s %s", tool.Name, tool.Method, tool.Path)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil || schema["type"] != "object" || strings.Contains(string(tool.InputSchema), "$ref") {
			t.Fatalf("%s: schema %s", tool.Name, tool.InputSchema)
		}
	}
	if len(seen) < 40 {
		t.Fatalf("only %d tools", len(seen))
	}
}

func normalise(t *testing.T, tools []mcpapi.Tool) any {
	t.Helper()
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExportRefusesBrokenSpecs(t *testing.T) {
	t.Parallel()
	op := func(x, extra string) string {
		return "paths:\n  /a/{id}:\n    post:\n      x-mcp: " + x + "\n      summary: S\n" + extra
	}
	for name, spec := range map[string]string{
		"yaml":           "paths: [",
		"no name":        op("{ entity: npc }", ""),
		"no entity":      op("{ tool: make }", ""),
		"bad param":      op("{ tool: make, entity: npc }", "      parameters:\n        - $ref: \"#/components/parameters/Nope\"\n"),
		"bad body":       op("{ tool: make, entity: npc }", "      requestBody:\n        content:\n          application/json:\n            schema: { $ref: \"#/components/schemas/Nope\" }\n"),
		"loop":           op("{ tool: make, entity: npc }", "      requestBody:\n        content:\n          application/json:\n            schema: { $ref: \"#/components/schemas/A\" }\ncomponents:\n  schemas:\n    A: { $ref: \"#/components/schemas/A\" }\n"),
		"bad nested":     op("{ tool: make, entity: npc }", "      requestBody:\n        content:\n          application/json:\n            schema: { properties: { a: { $ref: \"#/components/schemas/Nope\" } } }\n"),
		"unknown field":  "paths:\n  /a:\n    post:\n      x-mcp: { tools: [{ tool: go, kind: go, fields: [nope] }] }\n      requestBody:\n        content:\n          application/json:\n            schema: { properties: { a: { type: string } } }\n",
		"stray required": "paths:\n  /a:\n    post:\n      x-mcp: { tools: [{ tool: go, kind: go, fields: [a], required: [b] }] }\n      requestBody:\n        content:\n          application/json:\n            schema: { properties: { a: { type: string } } }\n",
		"loop in list":   op("{ tool: make, entity: npc }", "      requestBody:\n        content:\n          application/json:\n            schema: { allOf: [{ $ref: \"#/components/schemas/A\" }] }\ncomponents:\n  schemas:\n    A: { allOf: [{ $ref: \"#/components/schemas/A\" }] }\n"),
	} {
		if _, err := mcpapi.Export([]byte(spec)); err == nil {
			t.Fatalf("%s: exported", name)
		}
	}
	tools, err := mcpapi.Export([]byte(`paths:
  /a/{id}:
    parameters: []
    get:
      x-mcp: { tool: read }
      summary: Read
      description: One a.
      parameters:
        - { name: id, in: path, required: true, schema: { type: string } }
        - { name: q, in: query, schema: { type: string, example: x, discriminator: { propertyName: q } } }
        - { name: If-None-Match, in: header, schema: { type: string } }
    put:
      summary: Not a tool
`))
	if err != nil || len(tools) != 1 || tools[0].Description != "Read. One a." || !reflect.DeepEqual(tools[0].QueryParams, []string{"q"}) || strings.Contains(string(tools[0].InputSchema), "example") {
		t.Fatalf("tools %+v, %v", tools, err)
	}
}

// fakeAPI answers like the router: the campaign, a created entity, or a problem.
type fakeAPI struct {
	role  string
	seen  []*http.Request
	reply func(r *http.Request) (int, string)
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.seen = append(f.seen, r)
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/campaigns/") && strings.Count(r.URL.Path, "/") == 4 {
		if f.role == "" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"detail":"No such campaign, member or invite."}`)
			return
		}
		_, _ = io.WriteString(w, `{"myRole":"`+f.role+`"}`)
		return
	}
	code, body := f.reply(r)
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

type edits struct {
	err  error
	list []campaign.Edit
	got  campaign.EditFilter
}

func (e *edits) Edits(_ context.Context, _ campaign.CampaignID, f campaign.EditFilter) ([]campaign.Edit, error) {
	e.got = f
	return e.list, e.err
}

type identity string

func (i identity) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if i != "" {
		r.Header.Set("X-User-Id", string(i))
	}
	return http.DefaultTransport.RoundTrip(r)
}

func connect(t *testing.T, h http.Handler, who identity, name ...string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := mcp.NewClient(&mcp.Implementation{Name: append(name, "test-agent")[0], Version: "1"}, nil)
	s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: who}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args any) (string, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestToolsCallTheAPIAsTheCallerAndReportRevisions(t *testing.T) {
	t.Parallel()
	cid, npc, rev := uuid.New(), uuid.New(), uuid.New()
	api := &fakeAPI{role: "dm", reply: func(r *http.Request) (int, string) {
		client, _ := caller.MCPClient(r.Context())
		switch {
		case client != "test-agent":
			return 500, `{}`
		case r.Method == http.MethodDelete:
			return http.StatusNoContent, ""
		case strings.HasSuffix(r.URL.Path, "/npcs") && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			return http.StatusCreated, `{"id":"` + npc.String() + `","echo":` + string(body) + `}`
		case r.URL.Query().Get("q") == "gob":
			return 200, `[{"slug":"goblin"}]`
		}
		return http.StatusUnprocessableEntity, `{"detail":"Not that."}`
	}}
	found := &edits{list: []campaign.Edit{{RevisionID: rev, Revision: campaign.Revision{No: 3, Action: campaign.ActionUpdate}}}}
	s := connect(t, mcpapi.Handler(mcpapi.Options{API: api, Edits: found, Log: quiet}), "dm")

	text, failed := callTool(t, s, "create_npc", map[string]any{"campaignId": cid, "body": map[string]any{"name": "Hilda"}})
	if failed || !strings.Contains(text, `"echo":{"name":"Hilda"}`) || !strings.Contains(text, `"revision":{"id":"`+rev.String()+`","no":3,"action":"update"}`) {
		t.Fatalf("create: %s", text)
	}
	if found.got.EntityID != npc || found.got.EntityType != "npc" {
		t.Fatalf("revision looked up for %+v", found.got)
	}
	last := api.seen[len(api.seen)-1]
	if last.Header.Get("X-User-Id") != "dm" || last.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("request headers %v", last.Header)
	}
	other := uuid.New()
	if text, failed := callTool(t, s, "delete_npc", map[string]any{"campaignId": cid, "npcId": other}); failed || !strings.Contains(text, `"result":null`) || found.got.EntityID != other {
		t.Fatalf("delete: %s %+v", text, found.got)
	}
	if text, failed := callTool(t, s, "search_compendium", map[string]any{"kind": "monster", "q": "gob", "limit": 5}); failed || !strings.Contains(text, "goblin") || strings.Contains(text, "revision") {
		t.Fatalf("search: %s", text)
	}
	if !strings.Contains(api.seen[len(api.seen)-1].URL.RawQuery, "limit=5") {
		t.Fatalf("query %s", api.seen[len(api.seen)-1].URL.RawQuery)
	}
	for _, c := range []struct {
		tool string
		args any
		want string
	}{
		{"update_npc", map[string]any{"campaignId": cid, "body": map[string]any{}}, "Missing argument npcId."},
		{"list_npcs", map[string]any{"campaignId": cid}, "Not that."},
	} {
		if text, failed := callTool(t, s, c.tool, c.args); !failed || text != c.want {
			t.Fatalf("%s: %s", c.tool, text)
		}
	}
	found.err = errors.New("db down")
	if text, failed := callTool(t, s, "create_npc", map[string]any{"campaignId": cid, "body": map[string]any{"name": "Hilda"}}); failed || strings.Contains(text, "revision") {
		t.Fatalf("lost revision: %s", text)
	}
	api.role = "player"
	if text, failed := callTool(t, s, "list_npcs", map[string]any{"campaignId": cid}); !failed || text != "Only the campaign's DM can use Grimoire's tools on it." {
		t.Fatalf("player: %s", text)
	}
}

func TestToolsRefuseBadArgumentsAndBrokenAnswers(t *testing.T) {
	t.Parallel()
	api := &fakeAPI{role: "dm", reply: func(*http.Request) (int, string) { return 502, "bad gateway" }}
	h := mcpapi.Handler(mcpapi.Options{API: api, Edits: &edits{}, Log: quiet})
	s := connect(t, h, "dm")
	if text, failed := callTool(t, s, "list_campaigns", []int{1}); !failed || text != "The arguments must be a JSON object." {
		t.Fatalf("array: %s", text)
	}
	if text, failed := callTool(t, s, "list_campaigns", nil); !failed || text != "The request failed. Try again shortly." {
		t.Fatalf("broken: %s", text)
	}
	api.role = ""
	if text, failed := callTool(t, s, "list_npcs", map[string]any{"campaignId": 7}); !failed || text != "No such campaign, member or invite." {
		t.Fatalf("missing campaign: %s", text)
	}
	api.role = `"x`
	if text, failed := callTool(t, s, "list_npcs", map[string]any{"campaignId": 7}); !failed || !strings.Contains(text, "Only the campaign's DM") {
		t.Fatalf("garbled campaign: %s", text)
	}
}

func TestUnnamedClientsAreRecordedAsMCP(t *testing.T) {
	t.Parallel()
	var got string
	api := &fakeAPI{role: "dm", reply: func(r *http.Request) (int, string) {
		got, _ = caller.MCPClient(r.Context())
		return 200, "[]"
	}}
	s := connect(t, mcpapi.Handler(mcpapi.Options{API: api, Edits: &edits{}, Log: quiet}), "dm", "")
	if _, failed := callTool(t, s, "list_campaigns", nil); failed || got != "mcp" {
		t.Fatalf("client %q", got)
	}
}

func TestAToolCallWithoutArguments(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(mcpapi.Handler(mcpapi.Options{API: &fakeAPI{role: "dm", reply: func(*http.Request) (int, string) { return 200, "[]" }}, Edits: &edits{}, Log: quiet}))
	t.Cleanup(srv.Close)
	post := func(session, payload string) (string, string) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, strings.NewReader(payload))
		req.Header.Set("X-User-Id", "dm")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		got, _ := io.ReadAll(res.Body)
		return res.Header.Get("Mcp-Session-Id"), string(got)
	}
	session, _ := post("", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw","version":"1"}}}`)
	post(session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	_, body := post(session, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_campaigns"}}`)
	if !strings.Contains(body, `{\"result\":[]}`) {
		t.Fatalf("no arguments: %s", body)
	}
}

func TestEndpointNeedsAnIdentityAndPointsAtTheIssuer(t *testing.T) {
	t.Parallel()
	post := func(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	plain := mcpapi.Handler(mcpapi.Options{API: &fakeAPI{}, Edits: &edits{}, Log: quiet})
	rec := post(plain, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/mcp", strings.NewReader(body)))
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("plain: %d %v", rec.Code, rec.Header())
	}
	oauth := mcpapi.Handler(mcpapi.Options{API: &fakeAPI{}, Edits: &edits{}, Log: quiet, Issuer: "https://auth.example"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://grimoire.test/mcp", strings.NewReader(body))
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "play.example")
	rec = post(oauth, req)
	if want := `Bearer resource_metadata="https://play.example/.well-known/oauth-protected-resource"`; rec.Header().Get("WWW-Authenticate") != want {
		t.Fatalf("challenge %q", rec.Header().Get("WWW-Authenticate"))
	}
	for _, c := range []struct {
		tls  bool
		want string
	}{{false, "http://grimoire.test/mcp"}, {true, "https://grimoire.test/mcp"}} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://grimoire.test"+mcpapi.MetadataPath, nil)
		if c.tls {
			req.TLS = &tls.ConnectionState{}
		}
		rec := post(mcpapi.Metadata("https://auth.example"), req)
		var meta struct {
			Resource string   `json:"resource"`
			Servers  []string `json:"authorization_servers"`
		}
		if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&meta); err != nil || meta.Resource != c.want || meta.Servers[0] != "https://auth.example" {
			t.Fatalf("metadata %s", rec.Body.String())
		}
	}
}

// A write tool says how its write is kept: as a Revision of a Campaign's prep, as a Revision of a
// Library entry, or as something that waits for the DM. One that says none of these is no tool.
func TestExportTakesWritesThatAreKeptSomeOtherWay(t *testing.T) {
	t.Parallel()
	op := func(x string) []byte {
		return []byte("paths:\n  /a/{id}:\n    post:\n      x-mcp: " + x + "\n      summary: S\n      description: D.\n")
	}
	for keeps, note := range map[string]string{
		"library": "S. D. Every call saves a new Revision of the Library entry, named in the answer; restore_library_revision brings an earlier one back.",
		"pending": "S. D. Nothing changes until the DM confirms it in the app.",
	} {
		tools, err := mcpapi.Export(op("{ tool: make, keeps: " + keeps + " }"))
		if err != nil || len(tools) != 1 || tools[0].Keeps != keeps || tools[0].Entity != "" || tools[0].Description != note {
			t.Fatalf("keeps %s: %+v %v", keeps, tools, err)
		}
	}
	for _, x := range []string{"{ tool: make, keeps: forever }", "{ tool: make, keeps: \"\" }", "{ tool: make }"} {
		if _, err := mcpapi.Export(op(x)); err == nil {
			t.Fatalf("%s: exported", x)
		}
	}
	// A tool that only reads keeps nothing, and may not claim to.
	read := []byte("paths:\n  /a:\n    get:\n      x-mcp: { tool: look, keeps: library }\n      summary: S\n")
	if _, err := mcpapi.Export(read); err == nil {
		t.Fatal("a read that keeps a Revision: exported")
	}
}

// A write to a Library entry answers with the Revision it made, read back as the caller; a suggestion
// that waits for the DM says so.
func TestToolsReportLibraryRevisionsAndWhatIsPending(t *testing.T) {
	t.Parallel()
	entry, cid, faction := uuid.New(), uuid.New(), uuid.New()
	detail := `{"entry":{"id":"` + entry.String() + `"},"revisions":[{"no":4,"origin":"mcp","client":"test-agent"},{"no":3,"origin":"ui"}]}`
	lost := false
	api := &fakeAPI{role: "dm", reply: func(r *http.Request) (int, string) {
		path := r.URL.Path
		switch {
		case r.Header.Get("X-User-Id") != "aria":
			return 500, `{}`
		case r.Method == http.MethodGet && path == "/api/v1/library/"+entry.String():
			if lost {
				return http.StatusNotFound, `{"detail":"gone"}`
			}
			return 200, detail
		case r.Method == http.MethodPost && path == "/api/v1/library":
			return http.StatusCreated, `{"id":"` + entry.String() + `","name":"Bog Hag"}`
		case r.Method == http.MethodPut && (path == "/api/v1/library/"+entry.String() || path == "/api/v1/builders/roll-tables/"+entry.String()):
			return 200, `{"entry":{"id":"` + entry.String() + `"}}`
		case r.Method == http.MethodPost && path == "/api/v1/library/"+entry.String()+"/revisions/2/restore":
			return 200, `{"entry":{"id":"` + entry.String() + `"}}`
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/factions/"+faction.String()+"/standing-changes"):
			return http.StatusCreated, `{"id":"x","status":"pending"}`
		case r.Method == http.MethodGet && path == "/api/v1/library":
			return 200, `[]`
		}
		return http.StatusUnprocessableEntity, `{"detail":"Not that."}`
	}}
	s := connect(t, mcpapi.Handler(mcpapi.Options{API: api, Edits: &edits{}, Log: quiet}), "aria")
	made := `"libraryRevision":{"entryId":"` + entry.String() + `","no":4,"origin":"mcp","client":"test-agent"}`
	for tool, args := range map[string]map[string]any{
		"create_library_entry":     {"body": map[string]any{"kind": "creature", "name": "Bog Hag", "fields": []any{}}},
		"update_library_entry":     {"entryId": entry, "body": map[string]any{"name": "Bog Hag", "fields": []any{}}},
		"save_roll_table_build":    {"entryId": entry, "body": map[string]any{"dice": "1d6", "results": []any{}}},
		"restore_library_revision": {"entryId": entry, "revisionNo": 2},
	} {
		text, failed := callTool(t, s, tool, args)
		if failed || !strings.Contains(text, made) || strings.Contains(text, `"revision":`) || strings.Contains(text, "pending") {
			t.Fatalf("%s: %s", tool, text)
		}
	}
	// What it reads keeps nothing, and says nothing of a Revision.
	if text, failed := callTool(t, s, "list_library_entries", nil); failed || text != `{"result":[]}` {
		t.Fatalf("list: %s", text)
	}
	text, failed := callTool(t, s, "propose_standing_change", map[string]any{"campaignId": cid, "factionId": faction, "body": map[string]any{"delta": 2, "reason": "They saved the mill."}})
	if failed || text != `{"result":{"id":"x","status":"pending"},"pending":true}` {
		t.Fatalf("a suggestion: %s", text)
	}
	// A Revision that cannot be read back does not undo the write: the answer goes out without it.
	lost = true
	if text, failed := callTool(t, s, "update_library_entry", map[string]any{"entryId": entry, "body": map[string]any{"name": "Bog Hag", "fields": []any{}}}); failed || strings.Contains(text, "libraryRevision") || !strings.Contains(text, `"result":{"entry"`) {
		t.Fatalf("a lost Revision: %s", text)
	}
	lost = false
	detail = `{"entry":{},"revisions":[]}`
	if text, failed := callTool(t, s, "update_library_entry", map[string]any{"entryId": entry, "body": map[string]any{"name": "Bog Hag", "fields": []any{}}}); failed || strings.Contains(text, "libraryRevision") {
		t.Fatalf("an entry with no Revisions: %s", text)
	}
	detail = `not json`
	if text, failed := callTool(t, s, "update_library_entry", map[string]any{"entryId": entry, "body": map[string]any{"name": "Bog Hag", "fields": []any{}}}); failed || strings.Contains(text, "libraryRevision") {
		t.Fatalf("a garbled entry: %s", text)
	}
	// The suggestion's Campaign is still the DM's alone to send tools at.
	api.role = "player"
	if text, failed := callTool(t, s, "propose_standing_change", map[string]any{"campaignId": cid, "factionId": faction, "body": map[string]any{"delta": 2, "reason": "x"}}); !failed || text != "Only the campaign's DM can use Grimoire's tools on it." {
		t.Fatalf("a Player's agent: %s", text)
	}
}
