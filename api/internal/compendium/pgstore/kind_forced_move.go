package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// forcedMoveRows stores pushes and pulls in compendium.effect_forced_moves.
type forcedMoveRows struct{}

func (forcedMoveRows) kind() string { return effects.ForcedMove{}.Kind() }

//nolint:gosec // feet are bounded by a check
func (forcedMoveRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	f, _ := p.(effects.ForcedMove)
	return q.InsertEffectForcedMove(ctx, queries.InsertEffectForcedMoveParams{EffectID: at.effect, Ordinal: at.ordinal, Ft: int32(f.Ft), Toward: f.Toward})
}

func (forcedMoveRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectForcedMoves(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.ForcedMove{Ft: int(r.Ft), Toward: r.Toward}
	}
	return nil
}
