package app

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Limits on a search.
const (
	MinSearch = 2
	MaxSearch = 80
	// SearchPerKind is how many results each kind of thing gives.
	SearchPerKind = 5
)

// HomeRepository reads what the Dashboard and the search show. Every read is scoped to the caller: a
// Campaign they are a Member of, what a DM alone may see only where they are the DM, their own
// Library and the Shared Library, and their Friends.
type HomeRepository interface {
	LiveSessions(ctx context.Context, subject string) ([]domain.LiveSession, error)
	Needs(ctx context.Context, subject string) ([]domain.Need, error)
	// Search finds what has the query in its name, whatever its case, at most perKind of each kind of
	// thing: names that start with it first, the shortest of those first.
	Search(ctx context.Context, subject, query string, perKind int) ([]domain.Hit, error)
}

// Home runs the Dashboard and the universal search.
type Home struct {
	Repo HomeRepository
}

// Dashboard is the Sessions under way in the caller's Campaigns and what needs them before the next one.
func (h *Home) Dashboard(ctx context.Context, subject string) (domain.Dashboard, error) {
	live, err := h.Repo.LiveSessions(ctx, subject)
	if err != nil {
		return domain.Dashboard{}, err
	}
	needs, err := h.Repo.Needs(ctx, subject)
	return domain.Dashboard{Live: live, Needs: needs}, err
}

// Search finds what the caller may open whose name holds the query: the compendium, their own Library
// and the Shared Library, their Campaigns with the Characters in them and, where they are the DM, the
// NPCs, and their Friends.
func (h *Home) Search(ctx context.Context, subject, query string) ([]domain.Hit, error) {
	query = strings.TrimSpace(query)
	if n := utf8.RuneCountInString(query); n < MinSearch || n > MaxSearch {
		return nil, domain.ErrInvalid
	}
	return h.Repo.Search(ctx, subject, query, SearchPerKind)
}
