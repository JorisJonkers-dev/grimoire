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
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

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
