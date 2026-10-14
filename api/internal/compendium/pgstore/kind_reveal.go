package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// revealRows stores the Visibility Qualities a reveal strips in compendium.effect_reveals.
type revealRows struct{}

func (revealRows) kind() string { return effects.Reveal{}.Kind() }

func (revealRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	r, _ := p.(effects.Reveal)
	return q.InsertEffectReveal(ctx, queries.InsertEffectRevealParams{EffectID: at.effect, Ordinal: at.ordinal, Qualities: r.Qualities})
}

func (revealRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectReveals(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Reveal{Qualities: r.Qualities}
	}
	return nil
}
