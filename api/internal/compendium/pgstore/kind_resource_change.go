package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// resourceChangeRows stores Resource uses given or taken in compendium.effect_resource_changes.
type resourceChangeRows struct{}

func (resourceChangeRows) kind() string { return effects.ResourceChange{}.Kind() }

//nolint:gosec // the delta is bounded by a check
func (resourceChangeRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	r, _ := p.(effects.ResourceChange)
	return q.InsertEffectResourceChange(ctx, queries.InsertEffectResourceChangeParams{EffectID: at.effect, Ordinal: at.ordinal, ResourceSlug: r.Resource, Delta: int32(r.Delta)})
}

func (resourceChangeRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectResourceChanges(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.ResourceChange{Resource: r.ResourceSlug, Delta: int(r.Delta)}
	}
	return nil
}
