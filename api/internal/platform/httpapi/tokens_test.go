package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bearer makes a request with an Access Token.
func bearer(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mint(t *testing.T, public http.Handler, session, body string) (string, string) {
	t.Helper()
	rec := send(public, http.MethodPost, "/api/v1/account/access-tokens", session, "", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint: %d %s", rec.Code, rec.Body.String())
	}
	out := decode(t, rec)
	made, _ := out["accessToken"].(map[string]any)
	token, _ := out["token"].(string)
	id, _ := made["id"].(string)
	return token, id
}

// An Access Token acts as its Account within its scopes, shows only once, records when it was used and
// stops at once when revoked or expired; it can never manage the Account itself.
func TestAccessTokensOnREST(t *testing.T) {
	t.Parallel()
	trusted, public, _, now := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	reader, readerID := mint(t, public, aria, `{"name":"Notebook","scopes":["read"],"days":30}`)
	if !strings.HasPrefix(reader, "gmt_") || len(reader) != 47 {
		t.Fatalf("token = %q", reader)
	}
	list := send(public, http.MethodGet, "/api/v1/account/access-tokens", aria, "", "")
	if strings.Contains(list.Body.String(), reader) || !strings.Contains(list.Body.String(), `"name":"Notebook"`) || strings.Contains(list.Body.String(), "lastUsedAt") {
		t.Fatalf("list = %s", list.Body.String())
	}
	if rec := bearer(public, http.MethodGet, "/api/v1/campaigns", reader, ""); rec.Code != http.StatusOK {
		t.Fatalf("a read: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(send(public, http.MethodGet, "/api/v1/account/access-tokens", aria, "", "").Body.String(), "lastUsedAt") {
		t.Fatal("last use is not recorded")
	}
	if rec := bearer(public, http.MethodPost, "/api/v1/campaigns", reader, `{"name":"Morvain","displayName":"Aria"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a write with a read token: %d", rec.Code)
	}
	const aSet = "0190c7a8-0000-7000-8000-000000000041"
	builder, _ := mint(t, public, aria, `{"name":"Agent","scopes":["read","build"],"days":7}`)
	if rec := bearer(public, http.MethodPost, "/api/v1/campaigns", builder, `{"name":"Morvain","displayName":"Aria"}`); rec.Code != http.StatusCreated {
		t.Fatalf("a write with a build token: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/account/access-tokens", `{"name":"More","scopes":["read"],"days":1}`},
		{http.MethodPut, "/api/v1/account/password", `{"password":"a new long password"}`},
		{http.MethodPost, "/api/v1/account/two-step", ""},
		// Dice Sets are an Account's own, and sharing or approving one is a person's act: no token does it.
		{http.MethodPost, "/api/v1/dice-sets", `{"name":"Ember","design":{"dice":{}}}`},
		{http.MethodPut, "/api/v1/dice-sets/chosen", `{}`},
		{http.MethodPut, "/api/v1/dice-sets/" + aSet, `{"name":"Ember","design":{"dice":{}}}`},
		{http.MethodDelete, "/api/v1/dice-sets/" + aSet, ""},
		{http.MethodPut, "/api/v1/dice-sets/" + aSet + "/sharing", `{"sharing":"everyone"}`},
		{http.MethodPost, "/api/v1/dice-sets/" + aSet + "/copy", ""},
		{http.MethodPut, "/api/v1/dice-sets/" + aSet + "/image", "x"},
		{http.MethodDelete, "/api/v1/dice-sets/" + aSet + "/image", ""},
		{http.MethodPost, "/api/v1/admin/dice-sets/" + aSet + "/review", `{"approve":true,"picture":"0123456789ab"}`},
	} {
		if rec := bearer(public, c.method, c.path, builder, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with a token: %d", c.method, c.path, rec.Code)
		}
	}
	if rec := send(public, http.MethodDelete, "/api/v1/account/access-tokens/"+readerID, aria, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if rec := bearer(public, http.MethodGet, "/api/v1/campaigns", reader, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a revoked token: %d", rec.Code)
	}
	if rec := send(public, http.MethodDelete, "/api/v1/account/access-tokens/"+readerID, aria, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("revoke twice: %d", rec.Code)
	}
	bram := setUp(t, trusted, public, "bram")
	_, builderID := mint(t, public, aria, `{"name":"Spare","scopes":["play"],"days":1}`)
	if rec := send(public, http.MethodDelete, "/api/v1/account/access-tokens/"+builderID, bram, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("revoking someone else's token: %d", rec.Code)
	}
	if rec := bearer(public, http.MethodGet, "/api/v1/campaigns", "gmt_"+strings.Repeat("x", 43), ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a made-up token: %d", rec.Code)
	}
	now.pass(8 * 24 * time.Hour)
	if rec := bearer(public, http.MethodGet, "/api/v1/campaigns", builder, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("an expired token: %d", rec.Code)
	}
	for body, want := range map[string]int{
		`{"name":"  ","scopes":["read"],"days":1}`:       http.StatusUnprocessableEntity,
		`{"name":"x","scopes":["read"],"days":0}`:        http.StatusBadRequest,
		`{"name":"x","scopes":["admin"],"days":1}`:       http.StatusBadRequest,
		`{"name":"x","scopes":["read","read"],"days":1}`: http.StatusBadRequest,
	} {
		if rec := send(public, http.MethodPost, "/api/v1/account/access-tokens", aria, "", body); rec.Code != want {
			t.Errorf("mint %s = %d, want %d", body, rec.Code, want)
		}
	}
}

type withToken string

func (w withToken) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(w))
	return http.DefaultTransport.RoundTrip(r)
}

// MCP tools act as the token's Account, and a tool outside the token's scopes is refused.
func TestAccessTokensOnMCP(t *testing.T) {
	t.Parallel()
	trusted, public, _, _ := accountServers(t)
	aria := setUp(t, trusted, public, "aria")
	builder, _ := mint(t, public, aria, `{"name":"Agent","scopes":["read","build"],"days":7}`)
	rec := bearer(public, http.MethodPost, "/api/v1/campaigns", builder, `{"name":"Morvain","displayName":"Aria"}`)
	campaignID, _ := decode(t, rec)["id"].(string)
	reader, _ := mint(t, public, aria, `{"name":"Reader","scopes":["read"],"days":7}`)
	srv := httptest.NewServer(public)
	t.Cleanup(srv.Close)
	connect := func(token string) agent {
		c := mcp.NewClient(&mcp.Implementation{Name: "token-agent", Version: "1"}, nil)
		s, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: withToken(token)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return agent{t: t, s: s}
	}
	read := connect(reader)
	if out := read.must("list_campaigns", map[string]any{}); !strings.Contains(string(out.Result), "Morvain") {
		t.Fatalf("list as aria = %s", out.Result)
	}
	npc := map[string]any{"campaignId": campaignID, "body": map[string]any{"name": "Tamsin", "title": "Innkeeper", "disposition": "friendly"}}
	if _, refused := read.call("create_npc", npc); !strings.Contains(refused, "scopes do not allow") {
		t.Fatalf("a write with a read token = %q", refused)
	}
	connect(builder).must("create_npc", npc)
	if _, err := mcp.NewClient(&mcp.Implementation{Name: "x", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp", HTTPClient: &http.Client{Transport: withToken("gmt_" + strings.Repeat("y", 43))}}, nil); err == nil {
		t.Fatal("a made-up token connects")
	}
}

// Every operation names the scope group an Access Token needs for it, and every read is a Read.
func TestEveryOperationHasAScopeGroup(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile("../../../../openapi/v1/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	inPaths, method := false, ""
	lines := strings.Split(string(spec), "\n")
	for i, l := range lines {
		switch {
		case l == "paths:":
			inPaths = true
		case inPaths && l != "" && l[0] != ' ':
			inPaths = false
		case inPaths && strings.HasPrefix(l, "    ") && !strings.HasPrefix(l, "     ") && strings.HasSuffix(l, ":"):
			method = strings.TrimSuffix(strings.TrimSpace(l), ":")
		case inPaths && strings.HasPrefix(l, "      operationId: "):
			group := strings.TrimPrefix(lines[i+1], "      x-ogen-operation-group: ")
			if group == lines[i+1] || !map[string]bool{"Read": true, "Build": true, "Play": true, "Account": true}[group] {
				t.Errorf("%s has no scope group", l)
			}
			if method == "get" && group != "Read" {
				t.Errorf("%s is a read in group %s", l, group)
			}
		}
	}
}
