package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// reactsRows stores the reactions Effects give in compendium.effect_reactions.
type reactsRows struct{}

func (reactsRows) kind() string { return effects.Reacts{}.Kind() }

func (reactsRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	r, _ := p.(effects.Reacts)
	return q.InsertEffectReaction(ctx, queries.InsertEffectReactionParams{EffectID: at.effect, Ordinal: at.ordinal, Trigger: r.Trigger, Instruction: r.Instruction})
}

func (reactsRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectReactions(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Reacts{Trigger: r.Trigger, Instruction: r.Instruction}
	}
	return nil
}
