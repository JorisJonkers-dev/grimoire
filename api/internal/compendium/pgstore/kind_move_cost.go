package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// moveCostRows stores movement costs in compendium.effect_move_costs.
type moveCostRows struct{}

func (moveCostRows) kind() string { return effects.MoveCost{}.Kind() }

//nolint:gosec // the multiplier is bounded by a check
func (moveCostRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	m, _ := p.(effects.MoveCost)
	return q.InsertEffectMoveCost(ctx, queries.InsertEffectMoveCostParams{EffectID: at.effect, Ordinal: at.ordinal, Multiplier: int32(m.Multiplier)})
}

func (moveCostRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectMoveCosts(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.MoveCost{Multiplier: int(r.Multiplier)}
	}
	return nil
}
