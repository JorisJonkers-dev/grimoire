package mcpapi

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	campaign "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

//go:embed tools.json
var toolsJSON []byte

// Edits finds the Revisions a tool's write recorded.
type Edits interface {
	Edits(ctx context.Context, id campaign.CampaignID, f campaign.EditFilter) ([]campaign.Edit, error)
}

// Options wires the MCP endpoint to the API it calls.
type Options struct {
	// API is the generated router; every tool call is served by it as the caller.
	API     http.Handler
	Edits   Edits
	Version string
	Log     *slog.Logger
	// Issuer is the OAuth authorization server an agent signs in with; empty leaves discovery out.
	Issuer string
}

// Tools are the tools the spec declares; the generated file is checked by its tests.
func Tools() []Tool {
	var t []Tool
	_ = json.Unmarshal(toolsJSON, &t)
	return t
}

// Handler serves MCP over Streamable HTTP. Every request must carry the platform's identity.
func Handler(o Options) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "grimoire", Version: o.Version}, &mcp.ServerOptions{
		Instructions: "Grimoire runs D&D campaigns. Tools act as you, and campaign tools need you to be that campaign's DM. " +
			"Writes apply at once and return the Revision they recorded; undo_change reverts one.",
	})
	for _, t := range Tools() {
		srv.AddTool(&mcp.Tool{
			Name: t.Name, Description: t.Description, InputSchema: t.InputSchema,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: t.Method == http.MethodGet, DestructiveHint: destructive(t)},
		}, o.call(t))
	}
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{
		JSONResponse: true, SessionTimeout: 30 * time.Minute, Logger: o.Log,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(httpx.IdentityHeader) == "" {
			if o.Issuer != "" {
				w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+origin(r)+MetadataPath+`"`)
			}
			httpx.WriteProblem(w, http.StatusUnauthorized, "Unauthorized", "Sign in to continue.")
			return
		}
		stream.ServeHTTP(w, r)
	})
}

func destructive(t Tool) *bool {
	d := strings.HasPrefix(t.Name, "delete_")
	return &d
}

// result is what a tool returns: the API's answer, and the Revision a write recorded.
type result struct {
	Result   json.RawMessage `json:"result"`
	Revision *revision       `json:"revision,omitempty"`
}

type revision struct {
	ID     uuid.UUID `json:"id"`
	No     int       `json:"no"`
	Action string    `json:"action"`
}

func (o Options) call(t Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		subject, scopes := "", ""
		if req.Extra != nil {
			subject, scopes = req.Extra.Header.Get(httpx.IdentityHeader), req.Extra.Header.Get(httpx.ScopesHeader)
		}
		ctx = context.WithValue(ctx, scopesKey{}, scopes)
		args, ok := arguments(req.Params.Arguments)
		if !ok {
			return failure("The arguments must be a JSON object."), nil
		}
		path, missing := t.fill(args)
		if missing != "" {
			return failure("Missing argument " + missing + "."), nil
		}
		ctx = caller.WithMCPClient(ctx, client(req))
		if cid := text(args["campaignId"]); cid != "" {
			if msg := o.requireDM(ctx, subject, cid); msg != "" {
				return failure(msg), nil
			}
		}
		input, ok := t.body(args["body"])
		if !ok {
			return failure("The body must be a JSON object."), nil
		}
		status, body := o.serve(ctx, subject, t.Method, path, input)
		if status >= http.StatusBadRequest {
			return failure(detail(body)), nil
		}
		return success(o.answer(ctx, t, args, body)), nil
	}
}

// arguments reads a tool call's arguments as named JSON values.
func arguments(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	args := map[string]json.RawMessage{}
	if len(raw) == 0 {
		return args, true
	}
	return args, json.Unmarshal(raw, &args) == nil
}

// answer is a write's result with the Revision it recorded, when that can be found.
func (o Options) answer(ctx context.Context, t Tool, args map[string]json.RawMessage, body json.RawMessage) result {
	out := result{Result: body}
	if t.Entity == "" {
		return out
	}
	rev, err := o.revision(ctx, t, args, body)
	if err != nil {
		o.Log.ErrorContext(ctx, "mcp: find revision", "tool", t.Name, "error", err)
	}
	out.Revision = rev
	return out
}

// body is the request body: the caller's, or for a variant tool its defaults, then the caller's
// fields, then the preset it forces.
func (t Tool) body(given json.RawMessage) (json.RawMessage, bool) {
	if t.Preset == nil {
		return given, true
	}
	merged := map[string]any{}
	for k, v := range t.Defaults {
		merged[k] = v
	}
	if len(given) > 0 && json.Unmarshal(given, &merged) != nil {
		return nil, false
	}
	for k, v := range t.Preset {
		merged[k] = v
	}
	out, err := json.Marshal(merged)
	return out, err == nil
}

// fill puts the path and query arguments into the operation's URL, or names a missing one.
func (t Tool) fill(args map[string]json.RawMessage) (path, missing string) {
	path = t.Path
	for _, p := range t.PathParams {
		v := text(args[p])
		if v == "" {
			return "", p
		}
		path = strings.Replace(path, "{"+p+"}", url.PathEscape(v), 1)
	}
	q := url.Values{}
	for _, p := range t.QueryParams {
		if v := text(args[p]); v != "" {
			q.Set(p, v)
		}
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return path, ""
}

// text reads a string or number argument as it would appear in a URL.
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func client(req *mcp.CallToolRequest) string {
	if req.Session != nil {
		if p := req.Session.InitializeParams(); p != nil && p.ClientInfo != nil && p.ClientInfo.Name != "" {
			return p.ClientInfo.Name
		}
	}
	return "mcp"
}

// requireDM refuses campaign tools to anyone but that campaign's DM.
func (o Options) requireDM(ctx context.Context, subject, cid string) string {
	status, body := o.serve(ctx, subject, http.MethodGet, "/api/v1/campaigns/"+url.PathEscape(cid), nil)
	if status >= http.StatusBadRequest {
		return detail(body)
	}
	var c struct {
		MyRole string `json:"myRole"`
	}
	if json.Unmarshal(body, &c) != nil || c.MyRole != "dm" {
		return "Only the campaign's DM can use Grimoire's tools on it."
	}
	return ""
}

// scopesKey carries the scopes of the Access Token a tool call came with to every API call it makes.
type scopesKey struct{}

// serve runs one API call as the caller, within its Access Token's scopes, and returns its status and
// body.
func (o Options) serve(ctx context.Context, subject, method, path string, body json.RawMessage) (int, json.RawMessage) {
	r := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	r.Header.Set(httpx.IdentityHeader, subject)
	if scopes, _ := ctx.Value(scopesKey{}).(string); scopes != "" {
		r.Header.Set(httpx.ScopesHeader, scopes)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	o.API.ServeHTTP(rec, r)
	return rec.Code, bytes.TrimSpace(rec.Body.Bytes())
}

// revision finds the Revision a write just recorded, by the entity it returned or named.
func (o Options) revision(ctx context.Context, t Tool, args map[string]json.RawMessage, body json.RawMessage) (*revision, error) {
	var made struct {
		ID uuid.UUID `json:"id"`
	}
	if t.ID != "" {
		made.ID, _ = uuid.Parse(text(args[t.ID]))
	} else {
		_ = json.Unmarshal(body, &made)
	}
	cid, _ := uuid.Parse(text(args["campaignId"]))
	edits, err := o.Edits.Edits(ctx, campaign.CampaignID(cid), campaign.EditFilter{EntityType: campaign.EntityType(t.Entity), EntityID: made.ID, Limit: 1})
	if err != nil || len(edits) == 0 {
		return nil, errors.Join(err, errors.New("no revision recorded"))
	}
	return &revision{ID: edits[0].RevisionID, No: edits[0].No, Action: string(edits[0].Action)}, nil
}

func detail(body json.RawMessage) string {
	var p struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(body, &p) == nil && p.Detail != "" {
		return p.Detail
	}
	return "The request failed. Try again shortly."
}

func failure(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

func success(out result) *mcp.CallToolResult {
	if len(out.Result) == 0 {
		out.Result = json.RawMessage("null")
	}
	text, _ := json.Marshal(out)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}, StructuredContent: out}
}

// MetadataPath is where the OAuth protected resource metadata is served.
const MetadataPath = "/.well-known/oauth-protected-resource"

// Metadata tells an agent which authorization server issues tokens for /mcp.
func Metadata(issuer string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resource": origin(r) + "/mcp", "authorization_servers": []string{issuer}, "bearer_methods_supported": []string{"header"},
		})
	})
}

// origin is the public scheme and host the request came in on.
func origin(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
