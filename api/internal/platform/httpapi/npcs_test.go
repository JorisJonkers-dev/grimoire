package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func TestNPCRevisionsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/npcs"
	rec := call(h, http.MethodPost, base, "dm", `{"name":"Morvain","title":"Count","dmNotes":"Old","disposition":"hostile"}`)
	npcID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || npcID == "" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	one := base + "/" + npcID
	if rec := call(h, http.MethodGet, base, "dm", ""); !strings.Contains(rec.Body.String(), `"title":"Count"`) {
		t.Fatalf("list: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodPut, one, "dm", `{"name":"Morvain","dmNotes":"New","disposition":"neutral"}`); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, one+"/revisions/diff?from=1&to=2", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"field":"dmNotes","before":"Old","after":"New"`) {
		t.Fatalf("diff: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, one+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["dmNotes"] != "Old" {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, one+"/revisions", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"restoredFrom":1`) || !strings.Contains(rec.Body.String(), `"origin":"ui"`) {
		t.Fatalf("revisions: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeletedNPCsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/npcs"
	npcID, _ := decode(t, call(h, http.MethodPost, base, "dm", `{"name":"Morvain","disposition":"hostile"}`))["id"].(string)
	one := base + "/" + npcID
	if rec := call(h, http.MethodGet, one, "dm", ""); rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec := call(h, http.MethodGet, base+"/deleted", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Morvain") {
		t.Fatalf("deleted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "dm", ""); rec.Code != 200 || rec.Body.String() != "[]" {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player: %d", rec.Code)
	}
}

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
