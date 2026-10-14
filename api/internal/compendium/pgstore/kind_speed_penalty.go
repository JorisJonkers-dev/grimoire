package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// speedPenaltyRows stores speed penalties in compendium.effect_speed_penalties.
type speedPenaltyRows struct{}

func (speedPenaltyRows) kind() string { return effects.SpeedPenalty{}.Kind() }

//nolint:gosec // feet are bounded by a check
func (speedPenaltyRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.SpeedPenalty)
	return q.InsertEffectSpeedPenalty(ctx, queries.InsertEffectSpeedPenaltyParams{EffectID: at.effect, Ordinal: at.ordinal, Ft: int32(s.Ft)})
}

func (speedPenaltyRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectSpeedPenalties(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.SpeedPenalty{Ft: int(r.Ft)}
	}
	return nil
}
