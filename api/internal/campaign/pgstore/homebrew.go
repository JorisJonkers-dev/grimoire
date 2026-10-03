package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/classbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
)

// homebrew reads the designs of one kind a Campaign's Library adds and builds each; their designs were
// checked when saved.
func homebrew[D, B any](ctx context.Context, s *Store, id domain.CampaignID, kind string, build func(slug, name string, d D) B) ([]B, error) {
	rows, err := s.q.CampaignHomebrewDesigns(ctx, queries.CampaignHomebrewDesignsParams{CampaignID: uuid.UUID(id), Kind: kind})
	out := make([]B, 0, len(rows))
	for _, r := range rows {
		var d D
		_ = json.Unmarshal(r.Design, &d)
		out = append(out, build(spellbuild.Slug(r.ID.String()), r.Name, d))
	}
	return out, err
}

// HomebrewSubclasses builds the subclasses a Campaign's Library adds.
func (s *Store) HomebrewSubclasses(ctx context.Context, id domain.CampaignID) ([]subclassbuild.Subclass, error) {
	return homebrew(ctx, s, id, "subclass", subclassbuild.Compile)
}

// HomebrewClasses builds the classes a Campaign's Library adds.
func (s *Store) HomebrewClasses(ctx context.Context, id domain.CampaignID) ([]classbuild.Class, error) {
	return homebrew(ctx, s, id, "class", classbuild.Compile)
}
