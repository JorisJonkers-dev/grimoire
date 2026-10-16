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

type brokenTracks struct{ err error }

func (b brokenTracks) List(context.Context, caller.Caller, domain.CampaignID) (campaignapp.TracksView, error) {
	return campaignapp.TracksView{}, b.err
}

func (b brokenTracks) Create(context.Context, caller.Caller, domain.CampaignID, campaignapp.TrackInput) (domain.Track, error) {
	return domain.Track{}, b.err
}

func (b brokenTracks) Delete(context.Context, caller.Caller, domain.CampaignID, domain.TrackID) error {
	return b.err
}

func (b brokenTracks) Adjust(context.Context, caller.Caller, domain.CampaignID, domain.TrackID, *domain.CharacterID, int) (campaignapp.TrackAdjusted, error) {
	return campaignapp.TrackAdjusted{}, b.err
}

// A fault in the Track service is a fault, and nobody signed out gets anywhere.
func TestAFailingTrackServiceAnswers503WithoutItsCause(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/tracks"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.TrackService(brokenTracks{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"name":"A","scope":"party","min":0,"max":1,"start":0}`},
		{http.MethodDelete, one, ""},
		{http.MethodPost, one + "/adjustments", `{"delta":1}`},
	} {
		t.Run(o.method+" "+o.path, func(t *testing.T) {
			if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
				t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTrackOperationsWithoutAnIdentityAnswer401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListTracks(ctx, oas.ListTracksParams{}))
	add(hh.CreateTrack(ctx, &oas.TrackInput{}, oas.CreateTrackParams{}))
	add(hh.DeleteTrack(ctx, oas.DeleteTrackParams{}))
	add(hh.AdjustTrack(ctx, &oas.TrackAdjustment{}, oas.AdjustTrackParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
