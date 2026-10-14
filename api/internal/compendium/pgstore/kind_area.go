package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// areaRows stores templates in compendium.effect_areas.
type areaRows struct{}

func (areaRows) kind() string { return effects.Area{}.Kind() }

//nolint:gosec // sizes are bounded by checks
func (areaRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	a, _ := p.(effects.Area)
	return q.InsertEffectArea(ctx, queries.InsertEffectAreaParams{EffectID: at.effect, Ordinal: at.ordinal, Shape: string(a.Shape), SizeFt: int32(a.SizeFt), RangeFt: int32(a.RangeFt)})
}

func (areaRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectAreas(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Area{Shape: hex.Shape(r.Shape), SizeFt: int(r.SizeFt), RangeFt: int(r.RangeFt)}
	}
	return nil
}
