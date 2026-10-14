package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// grantFeatureRows stores granted features in compendium.effect_grants.
type grantFeatureRows struct{}

func (grantFeatureRows) kind() string { return effects.GrantFeature{}.Kind() }

func (grantFeatureRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	g, _ := p.(effects.GrantFeature)
	return q.InsertEffectGrant(ctx, queries.InsertEffectGrantParams{EffectID: at.effect, Ordinal: at.ordinal, Name: g.Name})
}

func (grantFeatureRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectGrants(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.GrantFeature{Name: r.Name}
	}
	return nil
}
