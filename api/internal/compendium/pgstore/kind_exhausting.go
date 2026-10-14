package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// exhaustingRows stores stacking levels in compendium.effect_exhaustion. The table has no cap: a cap
// comes from a homebrew condition's design, never from a stored row.
type exhaustingRows struct{}

func (exhaustingRows) kind() string { return effects.Exhausting{}.Kind() }

//nolint:gosec // every number is bounded by a check
func (exhaustingRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	e, _ := p.(effects.Exhausting)
	return q.InsertEffectExhaustion(ctx, queries.InsertEffectExhaustionParams{
		EffectID: at.effect, Ordinal: at.ordinal, D20PerLevel: int32(e.D20PerLevel), SpeedFtPerLevel: int32(e.SpeedFtPerLevel), DeathAt: int32(e.DeathAt),
	})
}

func (exhaustingRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectExhaustion(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Exhausting{D20PerLevel: int(r.D20PerLevel), SpeedFtPerLevel: int(r.SpeedFtPerLevel), DeathAt: int(r.DeathAt), MaxLevel: 0}
	}
	return nil
}
