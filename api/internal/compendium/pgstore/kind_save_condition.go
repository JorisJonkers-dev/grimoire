package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// saveConditionRows stores a save against a condition in compendium.effect_save_conditions.
type saveConditionRows struct{}

func (saveConditionRows) kind() string { return effects.SaveCondition{}.Kind() }

func (saveConditionRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.SaveCondition)
	return q.InsertEffectSaveCondition(ctx, queries.InsertEffectSaveConditionParams{EffectID: at.effect, Ordinal: at.ordinal, Ability: s.Ability, ConditionSlug: s.Slug})
}

func (saveConditionRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectSaveConditions(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.SaveCondition{Ability: r.Ability, Slug: r.ConditionSlug}
	}
	return nil
}
