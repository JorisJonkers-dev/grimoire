package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func campaignServer(t *testing.T, c httpapi.Campaigns, extra ...any) http.Handler {
	t.Helper()
	var cs httpapi.CharacterService
	var ns httpapi.NPCService
	var rs httpapi.RollService
	var ss httpapi.SessionService
	var hub httpapi.LiveHub
	var lm httpapi.LiveMembers
	var ms httpapi.MapService
	for _, e := range extra {
		switch v := e.(type) {
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
		}
	}
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: &fakeCompendium{}, Campaigns: c, Characters: cs, NPCs: ns, Rolls: rs, Sessions: ss, Hub: hub, LiveMembers: lm, Maps: ms, Log: quiet},
		RateLimit: 1000, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func realCampaigns(t *testing.T) http.Handler {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	repo := campaignpg.New(store.Pool())
	parts := []any{
		&app.Characters{
			Repo: repo, Compendium: &fakeCompendium{}, Combat: app.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now,
		},
		&app.NPCs{Repo: repo, Now: time.Now},
		httpapi.RollService(&playapp.Rolls{
			Repo: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo},
			Seed: rng.Seed, Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now,
		}),
	}
	return campaignServer(t, app.NewService(repo), append(parts, liveParts(t, store.Pool(), repo)...)...)
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

// campaignWithPlayer returns the id of a campaign run by "dm" that "player" joined.
func campaignWithPlayer(t *testing.T, h http.Handler) (string, map[string]any) {
	t.Helper()
	rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"Strahd","displayName":"Joris"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := decode(t, rec)["id"].(string)
	rec = call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/invites", "dm", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	invite := decode(t, rec)
	token, _ := invite["token"].(string)
	rec = call(h, http.MethodPost, "/api/v1/invites/accept", "player", `{"token":"`+token+`","displayName":"Ireena"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	return id, invite
}

func TestCampaignLifecycleOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, invite := campaignWithPlayer(t, h)
	token, _ := invite["token"].(string)
	rec := call(h, http.MethodPost, "/api/v1/invites/preview", "someone", `{"token":"`+token+`"}`)
	if rec.Code != 200 || decode(t, rec)["campaignName"] != "Strahd" {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns/"+id, "player", "")
	home := decode(t, rec)
	members, _ := home["members"].([]any)
	if rec.Code != 200 || home["myRole"] != "player" || len(members) != 2 {
		t.Fatalf("home: %d %v", rec.Code, home)
	}
	player, _ := members[1].(map[string]any)
	playerID, _ := player["id"].(string)
	if player["isMe"] != true {
		t.Fatalf("player row = %v", player)
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"name":"Barovia","ruleset":"srd-2014"}`)
	if rec.Code != 200 || decode(t, rec)["ruleset"] != "srd-2014" {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, "/api/v1/campaigns/"+id+"/members/"+playerID, "dm", `{"role":"dm"}`)
	if rec.Code != 200 || decode(t, rec)["role"] != "dm" {
		t.Fatalf("co-DM: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/invites", "player", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Joris") {
		t.Fatalf("invites: %d %s", rec.Code, rec.Body.String())
	}
	inviteID, _ := invite["id"].(string)
	if rec := call(h, http.MethodDelete, "/api/v1/campaigns/"+id+"/invites/"+inviteID, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, "/api/v1/campaigns/"+id+"/members/"+playerID, "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("leave: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id, "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("after leaving: %d", rec.Code)
	}
}

func TestCampaignListPages(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	for _, name := range []string{"One", "Two", "Three"} {
		if rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"`+name+`","displayName":"DM"}`); rec.Code != 201 {
			t.Fatalf("create: %d", rec.Code)
		}
	}
	rec := call(h, http.MethodGet, "/api/v1/campaigns?limit=2", "dm", "")
	page := decode(t, rec)
	items, _ := page["items"].([]any)
	next, _ := page["nextCursor"].(string)
	if rec.Code != 200 || len(items) != 2 || next == "" {
		t.Fatalf("page: %d %v", rec.Code, page)
	}
	rec = call(h, http.MethodGet, "/api/v1/campaigns?limit=2&cursor="+next, "dm", "")
	rest, _ := decode(t, rec)["items"].([]any)
	if rec.Code != 200 || len(rest) != 1 {
		t.Fatalf("second page: %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{"AAAA", "e30", "A"} {
		if rec := call(h, http.MethodGet, "/api/v1/campaigns?cursor="+bad, "dm", ""); rec.Code != http.StatusBadRequest {
			t.Errorf("cursor %q: %d", bad, rec.Code)
		}
	}
}

func TestEveryCampaignOperationRefusesOutsiders(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, invite := campaignWithPlayer(t, h)
	inviteID, _ := invite["id"].(string)
	home := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+id, "dm", ""))
	members, _ := home["members"].([]any)
	dmRow, _ := members[0].(map[string]any)
	dmID, _ := dmRow["id"].(string)
	base := "/api/v1/campaigns/" + id
	type op struct{ method, path, body string }
	dmOnly := []op{
		{http.MethodPatch, base, `{"name":"X"}`},
		{http.MethodPatch, base + "/members/" + dmID, `{"role":"player"}`},
		{http.MethodDelete, base + "/members/" + dmID, ""},
		{http.MethodGet, base + "/invites", ""},
		{http.MethodPost, base + "/invites", ""},
		{http.MethodDelete, base + "/invites/" + inviteID, ""},
	}
	for _, o := range append([]op{{http.MethodGet, base, ""}}, dmOnly...) {
		if rec := call(h, o.method, o.path, "stranger", o.body); rec.Code != http.StatusNotFound {
			t.Errorf("stranger %s %s: %d", o.method, o.path, rec.Code)
		}
		if rec := call(h, o.method, o.path, "", o.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", o.method, o.path, rec.Code)
		}
	}
	for _, o := range dmOnly {
		if rec := call(h, o.method, o.path, "player", o.body); rec.Code != http.StatusForbidden {
			t.Errorf("player %s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCampaignEdgeCasesOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	home := decode(t, call(h, http.MethodGet, base, "dm", ""))
	members, _ := home["members"].([]any)
	dmRow, _ := members[0].(map[string]any)
	dmID, _ := dmRow["id"].(string)
	if rec := call(h, http.MethodPatch, base+"/members/"+dmID, "dm", `{"role":"player"}`); rec.Code != http.StatusConflict {
		t.Errorf("last DM stepped down: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, "/api/v1/campaigns", "dm", `{"name":"  ","displayName":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("blank name: %d", rec.Code)
	}
	unknown := `{"token":"` + strings.Repeat("x", 43) + `"}`
	if rec := call(h, http.MethodPost, "/api/v1/invites/preview", "someone", unknown); rec.Code != http.StatusNotFound {
		t.Errorf("unknown invite: %d", rec.Code)
	}
	for _, path := range []string{"/api/v1/campaigns", "/api/v1/invites/preview", "/api/v1/invites/accept"} {
		if rec := call(h, http.MethodPost, path, "", `{}`); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
			t.Errorf("anonymous %s: %d", path, rec.Code)
		}
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous list: %d", rec.Code)
	}
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
	for i, r := range results {
		p, ok := r.(*oas.ProblemStatusCodeWithHeaders)
		if !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}

func liveParts(t *testing.T, pool *pgxpool.Pool, repo *campaignpg.Store) []any {
	t.Helper()
	members := playpg.CampaignMembers{Store: repo}
	hub := &live.Hub{Store: playpg.New(pool), Members: members, Owner: playpg.Owner{Pool: pool}, Now: time.Now, Log: quiet}
	t.Cleanup(hub.Shutdown)
	sessions := &playapp.Sessions{Repo: playpg.New(pool), Members: members, Live: hub, Now: time.Now}
	maps := &playapp.Maps{Repo: playpg.New(pool), Members: members, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now}
	return []any{httpapi.SessionService(sessions), httpapi.LiveHub(hub), httpapi.LiveMembers(members), httpapi.MapService(maps)}
}

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
