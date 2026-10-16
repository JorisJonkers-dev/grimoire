package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

type fixedHome struct {
	dash domain.Dashboard
	hits []domain.Hit
	err  error
}

func (f fixedHome) Dashboard(context.Context, string) (domain.Dashboard, error) { return f.dash, f.err }

func (f fixedHome) Search(context.Context, string, string) ([]domain.Hit, error) {
	return f.hits, f.err
}

// What the service answers is what goes out, field for field; a fault is a fault and says nothing of itself.
func TestTheDashboardAndTheSearchAsTheyGoOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	home := fixedHome{
		dash: domain.Dashboard{
			Live: []domain.LiveSession{{CampaignName: "Morvain", Number: 3, DM: true}, {CampaignName: "Saltmarsh", Number: 12}},
			Needs: []domain.Need{
				{Kind: domain.NeedLevelUp, Campaign: "Morvain", Title: "Kara can level up", Path: "/campaigns/x/characters/y/level-up"},
				{Kind: domain.NeedFriendRequests, Title: "1 Friend Request to answer", Path: "/friends"},
			},
		},
		hits: []domain.Hit{{Group: domain.GroupCompendium, Kind: "spell", Title: "Fireball", Preview: "Level 3 evocation spell", Path: "/compendium/spells/fireball"}},
	}
	h := campaignServer(t, brokenCampaigns{}, httpapi.HomeService(home))
	rec := call(h, http.MethodGet, "/api/v1/dashboard", "aria", "")
	none := "00000000-0000-0000-0000-000000000000"
	if want := `{"live":[{"campaignId":"` + none + `","campaign":"Morvain","sessionId":"` + none + `","number":3,"dm":true},{"campaignId":"` + none + `","campaign":"Saltmarsh","sessionId":"` + none + `","number":12,"dm":false}],` +
		`"needs":[{"kind":"level_up","campaign":"Morvain","title":"Kara can level up","path":"/campaigns/x/characters/y/level-up"},{"kind":"friend_requests","title":"1 Friend Request to answer","path":"/friends"}]}`; rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("the Dashboard: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, "/api/v1/search?q=fire", "aria", "")
	if want := `{"hits":[{"group":"compendium","kind":"spell","title":"Fireball","preview":"Level 3 evocation spell","path":"/compendium/spells/fireball"}]}`; rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("the search: %d %s", rec.Code, rec.Body.String())
	}

	// An Access Token reads both with read, and with nothing less.
	for _, path := range []string{"/api/v1/dashboard", "/api/v1/search?q=fire"} {
		if rec := withScopes(h, http.MethodGet, path, "aria", "read", ""); rec.Code != http.StatusOK {
			t.Errorf("a read token on %s: %d", path, rec.Code)
		}
		if rec := withScopes(h, http.MethodGet, path, "aria", "play build account", ""); rec.Code != http.StatusForbidden {
			t.Errorf("a token without read on %s: %d", path, rec.Code)
		}
	}

	broken := campaignServer(t, brokenCampaigns{}, httpapi.HomeService(fixedHome{err: errors.New("disk")}))
	for _, path := range []string{"/api/v1/dashboard", "/api/v1/search?q=fire"} {
		if rec := call(broken, http.MethodGet, path, "aria", ""); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	hh := &httpapi.Handler{Log: quiet}
	if res, _ := hh.GetDashboard(ctx); res.(*oas.ProblemStatusCodeWithHeaders).StatusCode != http.StatusUnauthorized {
		t.Errorf("the Dashboard signed out: %+v", res)
	}
	if res, _ := hh.Search(ctx, oas.SearchParams{Q: "fire"}); res.(*oas.ProblemStatusCodeWithHeaders).StatusCode != http.StatusUnauthorized {
		t.Errorf("a search signed out: %+v", res)
	}
}
