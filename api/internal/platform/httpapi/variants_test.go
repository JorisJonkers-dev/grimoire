package httpapi_test

import (
	"context"
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
