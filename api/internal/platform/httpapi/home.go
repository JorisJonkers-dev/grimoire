package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// HomeService is the Dashboard and the universal search.
type HomeService interface {
	Dashboard(ctx context.Context, subject string) (domain.Dashboard, error)
	Search(ctx context.Context, subject, query string) ([]domain.Hit, error)
}

// GetDashboard reads what the caller sees first.
func (h *Handler) GetDashboard(ctx context.Context) (oas.GetDashboardRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Home.Dashboard(ctx, id.Subject)
	if err != nil {
		h.Log.ErrorContext(ctx, "dashboard", "error", err)
		return unavailable(), nil
	}
	out := oas.Dashboard{Live: make([]oas.DashboardLiveSession, 0, len(d.Live)), Needs: make([]oas.DashboardNeed, 0, len(d.Needs))}
	for _, s := range d.Live {
		out.Live = append(out.Live, oas.DashboardLiveSession{
			CampaignId: oas.ID(s.Campaign), Campaign: oas.CampaignName(s.CampaignName), SessionId: oas.ID(s.Session), Number: int32(s.Number), Dm: s.DM, //nolint:gosec // a Session number
		})
	}
	for _, n := range d.Needs {
		need := oas.DashboardNeed{Kind: oas.DashboardNeedKind(n.Kind), Title: n.Title, Path: n.Path}
		if n.Campaign != "" {
			need.Campaign = oas.NewOptString(n.Campaign)
		}
		out.Needs = append(out.Needs, need)
	}
	return &oas.DashboardHeaders{Response: out}, nil
}

// Search finds what the caller may open by name.
func (h *Handler) Search(ctx context.Context, p oas.SearchParams) (oas.SearchRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	hits, err := h.Home.Search(ctx, id.Subject, p.Q)
	if errors.Is(err, domain.ErrInvalid) {
		return problem(http.StatusUnprocessableEntity, "Invalid", "Search for 2 to 80 characters."), nil
	}
	if err != nil {
		h.Log.ErrorContext(ctx, "search", "error", err)
		return unavailable(), nil
	}
	out := oas.SearchResults{Hits: make([]oas.SearchHit, 0, len(hits))}
	for _, hit := range hits {
		out.Hits = append(out.Hits, oas.SearchHit{Group: oas.SearchHitGroup(hit.Group), Kind: hit.Kind, Title: hit.Title, Preview: hit.Preview, Path: hit.Path})
	}
	return &oas.SearchResultsHeaders{Response: out}, nil
}
