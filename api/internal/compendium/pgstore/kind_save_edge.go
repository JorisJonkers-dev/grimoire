package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// saveEdgeRows stores saving throw edges in compendium.effect_save_edges.
type saveEdgeRows struct{}

func (saveEdgeRows) kind() string { return effects.SaveEdge{}.Kind() }

func (saveEdgeRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.SaveEdge)
	return q.InsertEffectSaveEdge(ctx, queries.InsertEffectSaveEdgeParams{EffectID: at.effect, Ordinal: at.ordinal, Ability: s.Ability, Mode: string(s.Mode)})
}

func (saveEdgeRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectSaveEdges(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.SaveEdge{Ability: r.Ability, Mode: effects.SaveMode(r.Mode)}
	}
	return nil
}
