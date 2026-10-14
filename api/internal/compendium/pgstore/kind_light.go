package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// lightRows stores light in compendium.effect_lights.
type lightRows struct{}

func (lightRows) kind() string { return effects.Light{}.Kind() }

//nolint:gosec // distances are bounded by checks
func (lightRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	l, _ := p.(effects.Light)
	return q.InsertEffectLight(ctx, queries.InsertEffectLightParams{EffectID: at.effect, Ordinal: at.ordinal, BrightFt: int32(l.BrightFt), DimFt: int32(l.DimFt)})
}

func (lightRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectLights(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Light{BrightFt: int(r.BrightFt), DimFt: int(r.DimFt)}
	}
	return nil
}
