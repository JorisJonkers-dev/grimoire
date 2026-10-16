package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func campaignServer(t *testing.T, c httpapi.Campaigns, extra ...any) http.Handler {
	t.Helper()
	var cs httpapi.CharacterService
	var ns httpapi.NPCService
	var comps httpapi.CompanionService
	var facs httpapi.FactionService
	var js httpapi.JournalService
	var rvs httpapi.RuleVariantService
	var rhs httpapi.RuleHookService
	var ts httpapi.TrackService
	var ds httpapi.DowntimeService
	var vs httpapi.VehicleService
	var hs httpapi.HomeService
	var rs httpapi.RollService
	var ss httpapi.SessionService
	var hub httpapi.LiveHub
	var lm httpapi.LiveMembers
	var ms httpapi.MapService
	var ps httpapi.PrepService
	var is httpapi.InventoryService
	var ls httpapi.LibraryService
	var sb httpapi.SpellBuilder
	var ib httpapi.ItemBuilder
	var scb httpapi.SubclassBuilder
	var cb httpapi.ClassBuilder
	var conditionb httpapi.ConditionBuilder
	var tableb httpapi.RollTableBuilder
	var monsterb httpapi.MonsterBuilder
	var backgroundb httpapi.BackgroundBuilder
	var featb httpapi.FeatBuilder
	var speciesb httpapi.SpeciesBuilder
	for _, e := range extra {
		switch v := e.(type) {
		case httpapi.LibraryService:
			ls = v
			sb, _ = v.(httpapi.SpellBuilder)
			ib, _ = v.(httpapi.ItemBuilder)
			scb, _ = v.(httpapi.SubclassBuilder)
			cb, _ = v.(httpapi.ClassBuilder)
			conditionb, _ = v.(httpapi.ConditionBuilder)
			tableb, _ = v.(httpapi.RollTableBuilder)
			monsterb, _ = v.(httpapi.MonsterBuilder)
			backgroundb, _ = v.(httpapi.BackgroundBuilder)
			featb, _ = v.(httpapi.FeatBuilder)
			speciesb, _ = v.(httpapi.SpeciesBuilder)
		case httpapi.InventoryService:
			is = v
		case httpapi.PrepService:
			ps = v
		case httpapi.MapService:
			ms = v
		case httpapi.SessionService:
			ss = v
		case httpapi.LiveHub:
			hub = v
		case httpapi.LiveMembers:
			lm = v
		case httpapi.RollService:
			rs = v
		case httpapi.CharacterService:
			cs = v
		case httpapi.NPCService:
			ns = v
		case httpapi.CompanionService:
			comps = v
		case httpapi.FactionService:
			facs = v
		case httpapi.JournalService:
			js = v
		case httpapi.RuleVariantService:
			rvs = v
		case httpapi.RuleHookService:
			rhs = v
		case httpapi.TrackService:
			ts = v
		case httpapi.DowntimeService:
			ds = v
		case httpapi.VehicleService:
			vs = v
		case httpapi.HomeService:
			hs = v
		}
	}
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Campaigns: c, Characters: cs, NPCs: ns, Companions: comps, Factions: facs, Journal: js, RuleVariants: rvs, RuleHooks: rhs, Tracks: ts, Downtime: ds, Vehicles: vs, Home: hs, Rolls: rs, Sessions: ss, Hub: hub, LiveMembers: lm, Maps: ms, Prep: ps, Inventory: is, Library: ls, Spells: sb, ItemBuilder: ib, Subclasses: scb, Classes: cb, SpeciesBuilds: speciesb, FeatBuilds: featb, BackgroundBuilds: backgroundb, ConditionBuilds: conditionb, RollTableBuilds: tableb, MonsterBuilds: monsterb, Log: quiet},
		RateLimit: 1000, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func call(h http.Handler, method, path, subject, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	if subject != "" {
		req.Header.Set("X-User-Id", subject)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type brokenCampaigns struct {
	err      error
	updateOK bool
}

func (b brokenCampaigns) Create(context.Context, caller.Caller, app.CreateInput) (domain.Detail, error) {
	return domain.Detail{}, b.err
}

func (b brokenCampaigns) List(context.Context, caller.Caller, *domain.ListCursor, int) ([]domain.Summary, error) {
	return nil, b.err
}

func (b brokenCampaigns) Get(context.Context, caller.Caller, domain.CampaignID) (domain.Detail, error) {
	return domain.Detail{}, b.err
}

func (b brokenCampaigns) Update(context.Context, caller.Caller, domain.CampaignID, app.UpdateInput) (domain.Campaign, error) {
	if b.updateOK {
		return domain.Campaign{}, nil
	}
	return domain.Campaign{}, b.err
}

func (b brokenCampaigns) SetRole(context.Context, caller.Caller, domain.CampaignID, domain.MemberID, domain.Role) (domain.Member, error) {
	return domain.Member{}, b.err
}

func (b brokenCampaigns) RemoveMember(context.Context, caller.Caller, domain.CampaignID, domain.MemberID) error {
	return b.err
}

func (b brokenCampaigns) CreateInvite(context.Context, caller.Caller, domain.CampaignID) (domain.NewInvite, error) {
	return domain.NewInvite{}, b.err
}

func (b brokenCampaigns) Invites(context.Context, caller.Caller, domain.CampaignID) ([]domain.Invite, error) {
	return nil, b.err
}

func (b brokenCampaigns) RevokeInvite(context.Context, caller.Caller, domain.CampaignID, domain.InviteID) error {
	return b.err
}

func (b brokenCampaigns) PreviewInvite(context.Context, string) (domain.InvitePreview, error) {
	return domain.InvitePreview{}, b.err
}

func (b brokenCampaigns) AcceptInvite(context.Context, caller.Caller, string, string) (domain.CampaignID, error) {
	return domain.CampaignID{}, b.err
}

func (b brokenCampaigns) Activity(context.Context, caller.Caller, domain.CampaignID) ([]domain.Edit, error) {
	return nil, b.err
}

func (b brokenCampaigns) LatestEdit(context.Context, caller.Caller, domain.CampaignID, domain.EntityType, uuid.UUID) (domain.Edit, error) {
	return domain.Edit{}, b.err
}

func (b brokenCampaigns) UndoPlan(context.Context, caller.Caller, domain.CampaignID, uuid.UUID) (domain.Edit, int, error) {
	return domain.Edit{}, 0, b.err
}

func TestUnexpectedCampaignErrorsAreHidden(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{err: errors.New("database on fire")})
	id := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	token := `"token":"` + strings.Repeat("x", 43) + `"`
	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/campaigns", ""},
		{http.MethodPost, "/api/v1/campaigns", `{"name":"A","displayName":"B"}`},
		{http.MethodGet, id, ""},
		{http.MethodPatch, id, `{"name":"A"}`},
		{http.MethodPatch, id + "/members/0190c7a8-0000-7000-8000-000000000002", `{"role":"dm"}`},
		{http.MethodDelete, id + "/members/0190c7a8-0000-7000-8000-000000000002", ""},
		{http.MethodGet, id + "/invites", ""},
		{http.MethodPost, id + "/invites", ""},
		{http.MethodDelete, id + "/invites/0190c7a8-0000-7000-8000-000000000003", ""},
		{http.MethodPost, "/api/v1/invites/preview", "{" + token + "}"},
		{http.MethodPost, "/api/v1/invites/accept", "{" + token + `,"displayName":"C"}`},
		{http.MethodGet, id + "/activity", ""},
		{http.MethodPost, id + "/activity/0190c7a8-0000-7000-8000-000000000004/undo", ""},
	}
	for _, c := range cases {
		rec := call(h, c.method, c.path, "u", c.body)
		if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "fire") {
			t.Errorf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	afterUpdate := campaignServer(t, brokenCampaigns{err: errors.New("gone"), updateOK: true})
	if rec := call(afterUpdate, http.MethodPatch, id, "u", `{"name":"A"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("reload after update: %d", rec.Code)
	}
}

func TestCampaignHandlersNeedAnIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(h.ListCampaigns(ctx, oas.ListCampaignsParams{}))
	add(h.CreateCampaign(ctx, &oas.CampaignCreate{}))
	add(h.GetCampaign(ctx, oas.GetCampaignParams{}))
	add(h.UpdateCampaign(ctx, &oas.CampaignUpdate{}, oas.UpdateCampaignParams{}))
	add(h.UpdateMember(ctx, &oas.MemberUpdate{}, oas.UpdateMemberParams{}))
	add(h.RemoveMember(ctx, oas.RemoveMemberParams{}))
	add(h.ListInvites(ctx, oas.ListInvitesParams{}))
	add(h.CreateInvite(ctx, oas.CreateInviteParams{}))
	add(h.RevokeInvite(ctx, oas.RevokeInviteParams{}))
	add(h.PreviewInvite(ctx, &oas.InviteToken{}))
	add(h.AcceptInvite(ctx, &oas.InviteAccept{}))
	add(h.ListActivity(ctx, oas.ListActivityParams{}))
	add(h.UndoChange(ctx, oas.UndoChangeParams{}))
	for i, r := range results {
		p, ok := r.(*oas.ProblemStatusCodeWithHeaders)
		if !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
