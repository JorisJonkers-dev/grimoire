package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// edgeRows stores edges in compendium.effect_edges.
type edgeRows struct{}

func (edgeRows) kind() string { return effects.Edge{}.Kind() }

func (edgeRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	e, _ := p.(effects.Edge)
	return q.InsertEffectEdge(ctx, queries.InsertEffectEdgeParams{
		EffectID: at.effect, Ordinal: at.ordinal, Against: e.Against, Advantage: e.Advantage, Reach: reachName(e.Range), SourceOnly: e.SourceOnly,
	})
}

func (edgeRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectEdges(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Edge{Against: r.Against, Advantage: r.Advantage, Range: reachOf(r.Reach), SourceOnly: r.SourceOnly}
	}
	return nil
}

// reachName stores a Range by name.
func reachName(r effects.Range) string {
	switch r {
	case effects.WithinFive:
		return "within_five"
	case effects.BeyondFive:
		return "beyond_five"
	case effects.AnyRange:
	}
	return "any"
}

// reachOf reads a stored reach; the database allows only the three names.
func reachOf(s string) effects.Range {
	switch s {
	case "within_five":
		return effects.WithinFive
	case "beyond_five":
		return effects.BeyondFive
	}
	return effects.AnyRange
}
