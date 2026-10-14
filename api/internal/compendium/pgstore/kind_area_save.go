package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// areaSaveRows stores an area's save in compendium.effect_area_saves.
type areaSaveRows struct{}

func (areaSaveRows) kind() string { return effects.AreaSave{}.Kind() }

func (areaSaveRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.AreaSave)
	return q.InsertEffectAreaSave(ctx, queries.InsertEffectAreaSaveParams{EffectID: at.effect, Ordinal: at.ordinal, Ability: s.Ability})
}

func (areaSaveRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectAreaSaves(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.AreaSave{Ability: r.Ability}
	}
	return nil
}
