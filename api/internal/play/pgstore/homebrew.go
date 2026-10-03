package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// SurfaceKinds lists the Surfaces a homebrew spell may lay down.
func (s *Store) SurfaceKinds(ctx context.Context) ([]string, error) {
	c, err := s.Surfaces(ctx)
	out := make([]string, 0, len(c))
	for _, k := range c.Kinds() {
		if k != surface.None {
			out = append(out, string(k))
		}
	}
	return out, err
}

// Homebrew builds the homebrew spells a Campaign sees, linked or through a Collection, at the Revision
// it is pinned to. A design the rules no longer run is left out rather than stopping the Session.
func (s *Store) Homebrew(ctx context.Context, campaign uuid.UUID) ([]spellbuild.Built, error) {
	rows, err := s.q.CampaignHomebrewSpells(ctx, campaign)
	if err != nil {
		return nil, err
	}
	kinds, err := s.SurfaceKinds(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]spellbuild.Built, 0, len(rows))
	for _, r := range rows {
		var d spellbuild.Design
		if json.Unmarshal(r.Design, &d) != nil {
			continue
		}
		if b, err := spellbuild.Build(spellbuild.Slug(r.ID.String()), r.Name, d, kinds); err == nil {
			out = append(out, b)
		}
	}
	return out, nil
}
