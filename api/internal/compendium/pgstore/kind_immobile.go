package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// immobileRows stores nothing beyond the component row: the kind is the whole Part.
type immobileRows struct{}

func (immobileRows) kind() string { return effects.Immobile{}.Kind() }

func (immobileRows) insert(context.Context, *queries.Queries, slot, effects.Part) error { return nil }

func (immobileRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	return bare(ctx, q, into, effects.Immobile{})
}
