package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type brokenNPCs struct{ err error }

func (b brokenNPCs) Create(context.Context, caller.Caller, domain.CampaignID, app.NPCInput) (domain.NPC, error) {
	return domain.NPC{}, b.err
}

func (b brokenNPCs) Update(context.Context, caller.Caller, domain.CampaignID, domain.NPCID, app.NPCInput) (domain.NPC, error) {
	return domain.NPC{}, b.err
}

func (b brokenNPCs) Delete(context.Context, caller.Caller, domain.CampaignID, domain.NPCID) error {
	return b.err
}

func (b brokenNPCs) Restore(context.Context, caller.Caller, domain.CampaignID, domain.NPCID, int) (domain.NPC, error) {
	return domain.NPC{}, b.err
}

func (b brokenNPCs) List(context.Context, caller.Caller, domain.CampaignID) ([]domain.NPC, error) {
	return nil, b.err
}

func (b brokenNPCs) Deleted(context.Context, caller.Caller, domain.CampaignID) ([]domain.DeletedNPC, error) {
	return nil, b.err
}

func (b brokenNPCs) Get(context.Context, caller.Caller, domain.CampaignID, domain.NPCID) (domain.NPC, error) {
	return domain.NPC{}, b.err
}

func (b brokenNPCs) Revisions(context.Context, caller.Caller, domain.CampaignID, domain.NPCID) ([]domain.Revision, error) {
	return nil, b.err
}

func (b brokenNPCs) Diff(context.Context, caller.Caller, domain.CampaignID, domain.NPCID, int, int) ([]domain.Change, error) {
	return nil, b.err
}

func TestNPCErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.NPCService(brokenNPCs{err: errors.New("disk")}))
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/npcs"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	body := `{"name":"A","disposition":"neutral"}`
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, body},
		{http.MethodGet, base + "/deleted", ""},
		{http.MethodGet, one, ""},
		{http.MethodPut, one, body},
		{http.MethodDelete, one, ""},
		{http.MethodGet, one + "/revisions", ""},
		{http.MethodGet, one + "/revisions/diff?from=1&to=2", ""},
		{http.MethodPost, one + "/revisions/1/restore", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListNpcs(ctx, oas.ListNpcsParams{}))
	add(hh.ListDeletedNpcs(ctx, oas.ListDeletedNpcsParams{}))
	add(hh.CreateNpc(ctx, &oas.NpcInput{}, oas.CreateNpcParams{}))
	add(hh.GetNpc(ctx, oas.GetNpcParams{}))
	add(hh.UpdateNpc(ctx, &oas.NpcInput{}, oas.UpdateNpcParams{}))
	add(hh.DeleteNpc(ctx, oas.DeleteNpcParams{}))
	add(hh.ListNpcRevisions(ctx, oas.ListNpcRevisionsParams{}))
	add(hh.DiffNpcRevisions(ctx, oas.DiffNpcRevisionsParams{}))
	add(hh.RestoreNpcRevision(ctx, oas.RestoreNpcRevisionParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
