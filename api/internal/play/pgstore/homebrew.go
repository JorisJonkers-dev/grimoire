package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
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

// Homebrew builds what a Campaign adds to the rules: the homebrew spells and conditions it sees,
// linked or through a Collection, at the Revision each is pinned to, and its exhaustion. A design the
// rules no longer run is left out rather than stopping the Session.
func (s *Store) Homebrew(ctx context.Context, campaign uuid.UUID) (live.Brew, error) {
	var out live.Brew
	variant, err := s.q.CampaignExhaustion(ctx, campaign)
	if err != nil {
		return out, err
	}
	out.Exhaustion, _ = conditionbuild.Exhaustion(variant)
	if out.Spells, err = s.homebrewSpells(ctx, campaign); err != nil {
		return out, err
	}
	out.Conditions, err = s.homebrewConditions(ctx, campaign)
	return out, err
}

func (s *Store) homebrewSpells(ctx context.Context, campaign uuid.UUID) ([]spellbuild.Built, error) {
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

func (s *Store) homebrewConditions(ctx context.Context, campaign uuid.UUID) ([]conditionbuild.Condition, error) {
	rows, err := s.q.CampaignHomebrewDesigns(ctx, queries.CampaignHomebrewDesignsParams{CampaignID: campaign, Kind: "condition"})
	out := make([]conditionbuild.Condition, 0, len(rows))
	for _, r := range rows {
		var d conditionbuild.Design
		_ = json.Unmarshal(r.Design, &d) // stored designs were checked when saved
		out = append(out, conditionbuild.Compile(spellbuild.Slug(r.ID.String()), r.Name, d))
	}
	return out, err
}
