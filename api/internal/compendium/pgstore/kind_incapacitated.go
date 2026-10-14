package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// incapacitatedRows stores nothing beyond the component row: the kind is the whole Part.
type incapacitatedRows struct{}

func (incapacitatedRows) kind() string { return effects.Incapacitated{}.Kind() }

func (incapacitatedRows) insert(context.Context, *queries.Queries, slot, effects.Part) error {
	return nil
}

func (incapacitatedRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	return bare(ctx, q, into, effects.Incapacitated{})
}
