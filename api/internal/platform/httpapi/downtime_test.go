package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type brokenDowntime struct{ err error }

func (b brokenDowntime) View(context.Context, caller.Caller, uuid.UUID) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

func (b brokenDowntime) Grant(context.Context, caller.Caller, uuid.UUID, *uuid.UUID, int) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

func (b brokenDowntime) AddRecipe(context.Context, caller.Caller, uuid.UUID, downtime.Recipe) (playdomain.CampaignRecipe, error) {
	return playdomain.CampaignRecipe{}, b.err
}

func (b brokenDowntime) RemoveRecipe(context.Context, caller.Caller, uuid.UUID, uuid.UUID) error {
	return b.err
}

func (b brokenDowntime) Spend(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.DowntimeActivity) (playapp.DowntimeView, error) {
	return playapp.DowntimeView{}, b.err
}

// A fault in the downtime service is a fault, and nobody signed out gets anywhere.
func TestAFailingDowntimeServiceAnswers503WithoutItsCause(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	h := campaignServer(t, brokenCampaigns{}, httpapi.DowntimeService(brokenDowntime{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base + "/downtime", ""},
		{http.MethodPost, base + "/downtime/grants", `{"days":1}`},
		{http.MethodPost, base + "/characters/0190c7a8-0000-7000-8000-000000000002/downtime", `{"activity":"work","days":1}`},
		{http.MethodPost, base + "/recipes", `{"name":"A","makes":"a","quantity":1,"days":1}`},
		{http.MethodDelete, base + "/recipes/0190c7a8-0000-7000-8000-000000000003", ""},
	} {
		t.Run(o.method+" "+o.path, func(t *testing.T) {
			if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
				t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDowntimeOperationsWithoutAnIdentityAnswer401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetDowntime(ctx, oas.GetDowntimeParams{}))
	add(hh.GrantDowntime(ctx, &oas.DowntimeGrant{}, oas.GrantDowntimeParams{}))
	add(hh.SpendDowntime(ctx, &oas.DowntimeActivity{}, oas.SpendDowntimeParams{}))
	add(hh.CreateRecipe(ctx, &oas.RecipeInput{}, oas.CreateRecipeParams{}))
	add(hh.DeleteRecipe(ctx, oas.DeleteRecipeParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
