package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// dispelRows stores nothing beyond the component row: the kind is the whole Part.
type dispelRows struct{}

func (dispelRows) kind() string { return effects.Dispel{}.Kind() }

func (dispelRows) insert(context.Context, *queries.Queries, slot, effects.Part) error { return nil }

func (dispelRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	return bare(ctx, q, into, effects.Dispel{})
}
