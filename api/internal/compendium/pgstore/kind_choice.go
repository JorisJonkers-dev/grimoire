package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// choiceRows stores nothing beyond the Part row: each mode is a named group of the Parts beneath.
type choiceRows struct{}

func (choiceRows) kind() string { return effects.Choice{}.Kind() }

func (choiceRows) insert(context.Context, *queries.Queries, slot, effects.Part) error { return nil }

func (choiceRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	return bare(ctx, q, into, effects.Choice{})
}

func (choiceRows) nest(_ effects.Part, groups []effects.Group) effects.Part {
	if len(groups) == 0 {
		return effects.Choice{Modes: nil}
	}
	return effects.Choice{Modes: groups}
}
