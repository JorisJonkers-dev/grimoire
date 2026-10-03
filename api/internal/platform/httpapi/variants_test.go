package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Every Member reads the Rule Variants with what each is at; the DM switches them.
func TestRuleVariantsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	path := "/api/v1/campaigns/" + id + "/rule-variants"
	list := func(who string) map[string]map[string]any {
		t.Helper()
		rec := call(h, http.MethodGet, path, who, "")
		var rows []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || rec.Code != http.StatusOK || len(rows) != 11 || rows[0]["slug"] != "flanking" {
			t.Fatalf("%s lists: %d %s", who, rec.Code, rec.Body.String())
		}
		out := map[string]map[string]any{}
		for _, row := range rows {
			out[row["slug"].(string)] = row
		}
		return out
	}
	seen := list("player")
	rests := seen["rests"]
	if options, _ := rests["options"].([]any); rests["value"] != "standard" || rests["name"] != "Rest lengths" || rests["automated"] != true || len(options) != 3 ||
		options[1].(map[string]any)["value"] != "gritty" || !strings.HasPrefix(options[1].(map[string]any)["label"].(string), "Gritty") || rests["description"] == "" {
		t.Fatalf("rests = %v", rests)
	}
	if seen["morale"]["automated"] != false || seen["flanking"]["value"] != "off" {
		t.Fatalf("to begin with = %v %v", seen["morale"], seen["flanking"])
	}
	if rec := call(h, http.MethodPut, path, "dm", `{"choices":[{"slug":"flanking","value":"on"},{"slug":"rests","value":"gritty"}]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
	}
	if seen := list("player"); seen["flanking"]["value"] != "on" || seen["rests"]["value"] != "gritty" || seen["morale"]["value"] != "off" {
		t.Fatalf("after switching = %v", seen)
	}
	for name, c := range map[string]struct {
		who, method, body string
		want              int
	}{
		"signed out lists":       {"", http.MethodGet, "", http.StatusUnauthorized},
		"signed out switches":    {"", http.MethodPut, `{"choices":[]}`, http.StatusUnauthorized},
		"a stranger lists":       {"stranger", http.MethodGet, "", http.StatusNotFound},
		"a player switches":      {"player", http.MethodPut, `{"choices":[{"slug":"flanking","value":"off"}]}`, http.StatusForbidden},
		"a value it cannot be":   {"dm", http.MethodPut, `{"choices":[{"slug":"morale","value":"on"},{"slug":"rests","value":"heroic"}]}`, http.StatusUnprocessableEntity},
		"a variant not there":    {"dm", http.MethodPut, `{"choices":[{"slug":"spell-points","value":"on"}]}`, http.StatusUnprocessableEntity},
		"a choice with no value": {"dm", http.MethodPut, `{"choices":[{"slug":"morale","value":""}]}`, http.StatusBadRequest},
		"no choices at all":      {"dm", http.MethodPut, `{}`, http.StatusBadRequest},
	} {
		if rec := call(h, c.method, path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	// An Access Token that may only read cannot switch; one that may build can.
	if rec := withScopes(h, http.MethodPut, path, "dm", "read play", `{"choices":[{"slug":"morale","value":"on"}]}`); rec.Code != http.StatusForbidden {
		t.Errorf("a token that cannot build switches: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPut, path, "dm", "read build", `{"choices":[{"slug":"morale","value":"on"}]}`); rec.Code != http.StatusNoContent {
		t.Errorf("a token that may build switches: %d", rec.Code)
	}
	if seen := list("dm"); seen["flanking"]["value"] != "on" || seen["rests"]["value"] != "gritty" || seen["morale"]["value"] != "on" {
		t.Fatalf("at the end = %v", seen)
	}
}

type brokenVariants struct{ err error }

func (b brokenVariants) List(context.Context, caller.Caller, domain.CampaignID) ([]campaignapp.RuleVariantView, error) {
	return nil, b.err
}

func (b brokenVariants) Set(context.Context, caller.Caller, domain.CampaignID, variants.Set) error {
	return b.err
}

// A fault in the Rule Variant service is a fault, and nobody signed out gets anywhere.
func TestRuleVariantErrors(t *testing.T) {
	t.Parallel()
	path := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/rule-variants"
	h := campaignServer(t, brokenCampaigns{}, httpapi.RuleVariantService(brokenVariants{err: errors.New("disk")}))
	if rec := call(h, http.MethodGet, path, "u", ""); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
		t.Errorf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, path, "u", `{"choices":[]}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("set: %d %s", rec.Code, rec.Body.String())
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListRuleVariants(ctx, oas.ListRuleVariantsParams{}))
	add(hh.SetRuleVariants(ctx, &oas.RuleVariantChoices{}, oas.SetRuleVariantsParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
