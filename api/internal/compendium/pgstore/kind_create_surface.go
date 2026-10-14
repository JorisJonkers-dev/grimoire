package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// createSurfaceRows stores Surfaces laid on an area in compendium.effect_surfaces.
type createSurfaceRows struct{}

func (createSurfaceRows) kind() string { return effects.CreateSurface{}.Kind() }

//nolint:gosec // rounds are bounded by a check
func (createSurfaceRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	c, _ := p.(effects.CreateSurface)
	return q.InsertEffectSurface(ctx, queries.InsertEffectSurfaceParams{EffectID: at.effect, Ordinal: at.ordinal, Surface: string(c.Surface), Rounds: int32(c.Rounds)})
}

func (createSurfaceRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectSurfaces(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.CreateSurface{Surface: surface.Kind(r.Surface), Rounds: int(r.Rounds)}
	}
	return nil
}
