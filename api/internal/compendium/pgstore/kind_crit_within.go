package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// critWithinRows stores close-range Critical Hits in compendium.effect_crits.
type critWithinRows struct{}

func (critWithinRows) kind() string { return effects.CritWithin{}.Kind() }

//nolint:gosec // feet are bounded by a check
func (critWithinRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	c, _ := p.(effects.CritWithin)
	return q.InsertEffectCrit(ctx, queries.InsertEffectCritParams{EffectID: at.effect, Ordinal: at.ordinal, Feet: int32(c.Feet)})
}

func (critWithinRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectCrits(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.CritWithin{Feet: int(r.Feet)}
	}
	return nil
}
