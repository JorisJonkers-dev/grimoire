package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// counterRows stores counters in compendium.effect_counters.
type counterRows struct{}

func (counterRows) kind() string { return effects.Counter{}.Kind() }

//nolint:gosec // the range is bounded by a check
func (counterRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	c, _ := p.(effects.Counter)
	return q.InsertEffectCounter(ctx, queries.InsertEffectCounterParams{EffectID: at.effect, Ordinal: at.ordinal, RangeFt: int32(c.RangeFt)})
}

func (counterRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectCounters(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Counter{RangeFt: int(r.RangeFt)}
	}
	return nil
}
