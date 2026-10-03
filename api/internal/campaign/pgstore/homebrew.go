package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
)

// HomebrewSubclasses builds the subclasses a Campaign's Library adds; their designs were checked when
// saved.
func (s *Store) HomebrewSubclasses(ctx context.Context, id domain.CampaignID) ([]subclassbuild.Subclass, error) {
	rows, err := s.q.CampaignHomebrewSubclasses(ctx, uuid.UUID(id))
	out := make([]subclassbuild.Subclass, 0, len(rows))
	for _, r := range rows {
		var d subclassbuild.Design
		_ = json.Unmarshal(r.Design, &d)
		out = append(out, subclassbuild.Compile(spellbuild.Slug(r.ID.String()), r.Name, d))
	}
	return out, err
}
