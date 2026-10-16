package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// withScopes calls as an Access Token with those scopes would.
func withScopes(h http.Handler, method, path, subject, scopes, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	req.Header.Set("X-User-Id", subject)
	req.Header.Set(httpx.ScopesHeader, scopes)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type brokenFactions struct{ err error }

func (b brokenFactions) List(context.Context, caller.Caller, domain.CampaignID) ([]campaignapp.FactionView, error) {
	return nil, b.err
}

func (b brokenFactions) Create(context.Context, caller.Caller, domain.CampaignID, campaignapp.FactionInput) (campaignapp.FactionView, error) {
	return campaignapp.FactionView{}, b.err
}

func (b brokenFactions) Update(context.Context, caller.Caller, domain.CampaignID, domain.FactionID, campaignapp.FactionInput) error {
	return b.err
}

func (b brokenFactions) Delete(context.Context, caller.Caller, domain.CampaignID, domain.FactionID) error {
	return b.err
}

func (b brokenFactions) Propose(context.Context, caller.Caller, domain.CampaignID, domain.FactionID, campaignapp.Suggestion) (domain.StandingChange, error) {
	return domain.StandingChange{}, b.err
}

func (b brokenFactions) Decide(context.Context, caller.Caller, domain.CampaignID, domain.ChangeID, campaignapp.Decision) error {
	return b.err
}

// fixedFactions answers with a Faction that has a Personal Standing and a change for one Character.
type fixedFactions struct{ brokenFactions }

func (fixedFactions) List(context.Context, caller.Caller, domain.CampaignID) ([]campaignapp.FactionView, error) {
	score, hero := 25, domain.CharacterID(uuid.MustParse("0190c7a8-0000-7000-8000-0000000000a1"))
	return []campaignapp.FactionView{{
		Faction: domain.Faction{ID: uuid.MustParse("0190c7a8-0000-7000-8000-0000000000f1"), Name: "The Watch", Score: -20}, Tier: "unfriendly", DM: true,
		Personal: []campaignapp.Personal{{CharacterID: hero, Character: "Tamsin", Tier: "friendly", Score: &score}, {CharacterID: hero, Character: "Tamsin", Tier: "friendly"}},
		Changes:  []campaignapp.Change{{ID: uuid.New(), CharacterID: &hero, Rose: true, Reason: "A favour.", Status: domain.ChangeConfirmed}},
	}}, nil
}

func (fixedFactions) Propose(_ context.Context, _ caller.Caller, _ domain.CampaignID, _ domain.FactionID, g campaignapp.Suggestion) (domain.StandingChange, error) {
	return domain.StandingChange{ID: uuid.New(), Character: g.Character, Delta: g.Delta, Reason: g.Reason, ShareReason: g.ShareReason, Status: domain.ChangePending, Origin: "ui"}, nil
}

// A Personal Standing and a change for one Character go over the wire with whose they are.
func TestPersonalStandingOverHTTP(t *testing.T) {
	t.Parallel()
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/factions"
	h := campaignServer(t, brokenCampaigns{}, httpapi.FactionService(fixedFactions{}))
	rec := call(h, http.MethodGet, c, "u", "")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"characterId":"0190c7a8-0000-7000-8000-0000000000a1","character":"Tamsin","tier":"friendly","score":25`) ||
		!strings.Contains(body, `"characterId":"0190c7a8-0000-7000-8000-0000000000a1","character":"Tamsin","tier":"friendly"}`) || strings.Count(body, `"characterId"`) != 3 || !strings.Contains(body, `"score":-20`) {
		t.Fatalf("list: %d %s", rec.Code, body)
	}
	rec = call(h, http.MethodPost, c+"/0190c7a8-0000-7000-8000-0000000000f1/standing-changes", "u", `{"characterId":"0190c7a8-0000-7000-8000-0000000000a1","delta":-5,"reason":"Rude.","shareReason":true}`)
	if got := decode(t, rec); rec.Code != http.StatusCreated || got["characterId"] != "0190c7a8-0000-7000-8000-0000000000a1" || got["rose"] != false || got["status"] != "pending" {
		t.Fatalf("propose for a Character: %d %s", rec.Code, rec.Body.String())
	}
}

// A fault in the Faction service is a fault, and nobody signed out gets anywhere.
func TestFactionErrors(t *testing.T) {
	t.Parallel()
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	one := c + "/factions/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.FactionService(brokenFactions{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, c + "/factions", ""},
		{http.MethodPost, c + "/factions", `{"name":"A"}`},
		{http.MethodPut, one, `{"name":"A"}`},
		{http.MethodDelete, one, ""},
		{http.MethodPost, one + "/standing-changes", `{"delta":5,"reason":"A"}`},
		{http.MethodPost, c + "/standing-changes/0190c7a8-0000-7000-8000-000000000003/decision", `{"confirm":true,"delta":5,"reason":"A","shareReason":true}`},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListFactionArchetypes(ctx))
	add(hh.ListFactions(ctx, oas.ListFactionsParams{}))
	add(hh.CreateFaction(ctx, &oas.FactionInput{}, oas.CreateFactionParams{}))
	add(hh.UpdateFaction(ctx, &oas.FactionInput{}, oas.UpdateFactionParams{}))
	add(hh.DeleteFaction(ctx, oas.DeleteFactionParams{}))
	add(hh.ProposeStandingChange(ctx, &oas.StandingChangeInput{}, oas.ProposeStandingChangeParams{}))
	add(hh.DecideStandingChange(ctx, &oas.StandingDecision{}, oas.DecideStandingChangeParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
