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

// A DM builds a fumble table, links it to the Campaign and hooks it to a natural 1; every Member reads
// the Campaign's own Rule Variants, and only the DM authors them.
func TestRuleHooksOverHTTP(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/rule-hooks"
	table := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"table","name":"Fumbles","fields":[]}`))["id"].(string)
	if rec := call(h, http.MethodPut, "/api/v1/builders/roll-tables/"+table, "dm", fumbleDesign); rec.Code != http.StatusOK {
		t.Fatalf("save the table: %d %s", rec.Code, rec.Body.String())
	}
	unlinked := `{"name":"Critical fumbles","hook":"natural-1","rollTableId":"` + table + `"}`
	if rec := call(h, http.MethodPost, base, "dm", unlinked); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "not one this Campaign sees") {
		t.Fatalf("a table not linked yet: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/library", "dm", `{"entryId":"`+table+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("link: %d %s", rec.Code, rec.Body.String())
	}
	rec := call(h, http.MethodPost, base, "dm", unlinked)
	fumble := decode(t, rec)
	fumbleID, _ := fumble["id"].(string)
	if rec.Code != http.StatusCreated || fumble["name"] != "Critical fumbles" || fumble["hook"] != "natural-1" || fumble["rollTableId"] != table || fumble["effect"] != nil {
		t.Fatalf("create with a table: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base, "dm", `{"name":"Winded","hook":"drop-to-0","effect":"exhaustion"}`)
	if got := decode(t, rec); rec.Code != http.StatusCreated || got["effect"] != "exhaustion" || got["rollTableId"] != nil || got["tableName"] != nil {
		t.Fatalf("create with an Effect: %d %s", rec.Code, rec.Body.String())
	}

	seen := decode(t, call(h, http.MethodGet, base, "player", ""))
	hooks, _ := seen["hooks"].([]any)
	if seen["dm"] != false || len(hooks) != 2 || hooks[0].(map[string]any)["tableName"] != "Fumbles" || hooks[1].(map[string]any)["effect"] != "exhaustion" ||
		len(seen["tables"].([]any)) != 0 || len(seen["points"].([]any)) != 5 || seen["points"].([]any)[0].(map[string]any)["label"] != "On a natural 1 on an attack roll" {
		t.Fatalf("a Player's list = %v", seen)
	}
	kept := decode(t, call(h, http.MethodGet, base, "dm", ""))
	if tables, _ := kept["tables"].([]any); kept["dm"] != true || len(tables) != 1 || tables[0].(map[string]any)["id"] != table || tables[0].(map[string]any)["name"] != "Fumbles" {
		t.Fatalf("the DM's list = %v", kept)
	}

	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"signed out lists":      {"", http.MethodGet, base, "", http.StatusUnauthorized},
		"signed out authors":    {"", http.MethodPost, base, `{"name":"A","hook":"rest","effect":"prone"}`, http.StatusUnauthorized},
		"signed out removes":    {"", http.MethodDelete, base + "/" + fumbleID, "", http.StatusUnauthorized},
		"a stranger lists":      {"stranger", http.MethodGet, base, "", http.StatusNotFound},
		"a player authors":      {"player", http.MethodPost, base, `{"name":"Mine","hook":"rest","effect":"prone"}`, http.StatusForbidden},
		"a player removes":      {"player", http.MethodDelete, base + "/" + fumbleID, "", http.StatusForbidden},
		"no such hook point":    {"dm", http.MethodPost, base, `{"name":"A","hook":"on-hit","effect":"prone"}`, http.StatusBadRequest},
		"no outcome":            {"dm", http.MethodPost, base, `{"name":"A","hook":"rest"}`, http.StatusUnprocessableEntity},
		"both outcomes":         {"dm", http.MethodPost, base, `{"name":"A","hook":"rest","effect":"prone","rollTableId":"` + table + `"}`, http.StatusUnprocessableEntity},
		"a blank name":          {"dm", http.MethodPost, base, `{"name":" ","hook":"rest","effect":"prone"}`, http.StatusUnprocessableEntity},
		"the removal of no one": {"dm", http.MethodDelete, base + "/" + uuid.NewString(), "", http.StatusNotFound},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := withScopes(h, http.MethodPost, base, "dm", "read play", `{"name":"A","hook":"rest","effect":"prone"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot build authors: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, base+"/"+fumbleID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if after := decode(t, call(h, http.MethodGet, base, "dm", "")); len(after["hooks"].([]any)) != 1 || after["hooks"].([]any)[0].(map[string]any)["name"] != "Winded" {
		t.Fatalf("after removing = %v", after)
	}
}

type brokenHooks struct{ err error }

func (b brokenHooks) List(context.Context, caller.Caller, domain.CampaignID) (campaignapp.RuleHooksView, error) {
	return campaignapp.RuleHooksView{}, b.err
}

func (b brokenHooks) Create(context.Context, caller.Caller, domain.CampaignID, campaignapp.RuleHookInput) (domain.RuleHook, error) {
	return domain.RuleHook{}, b.err
}

func (b brokenHooks) Delete(context.Context, caller.Caller, domain.CampaignID, domain.HookID) error {
	return b.err
}

// A fault in the hook service is a fault, and nobody signed out gets anywhere.
func TestRuleHookErrors(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/rule-hooks"
	h := campaignServer(t, brokenCampaigns{}, httpapi.RuleHookService(brokenHooks{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"name":"A","hook":"rest","effect":"prone"}`},
		{http.MethodDelete, base + "/0190c7a8-0000-7000-8000-000000000002", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListRuleHooks(ctx, oas.ListRuleHooksParams{}))
	add(hh.CreateRuleHook(ctx, &oas.RuleHookInput{}, oas.CreateRuleHookParams{}))
	add(hh.DeleteRuleHook(ctx, oas.DeleteRuleHookParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
