package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// manualRows stores Manual Effects in compendium.effect_manual.
type manualRows struct{}

func (manualRows) kind() string { return effects.Manual{}.Kind() }

func (manualRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	m, _ := p.(effects.Manual)
	return q.InsertEffectManual(ctx, queries.InsertEffectManualParams{EffectID: at.effect, Ordinal: at.ordinal, Instruction: m.Instruction})
}

func (manualRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectManual(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Manual{Instruction: r.Instruction}
	}
	return nil
}
