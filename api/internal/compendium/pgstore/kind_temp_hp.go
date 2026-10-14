package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// tempHPRows stores temporary hit points in compendium.effect_temp_hp.
type tempHPRows struct{}

func (tempHPRows) kind() string { return effects.TempHP{}.Kind() }

//nolint:gosec // the amount is bounded by a check
func (tempHPRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	t, _ := p.(effects.TempHP)
	return q.InsertEffectTempHP(ctx, queries.InsertEffectTempHPParams{EffectID: at.effect, Ordinal: at.ordinal, Amount: int32(t.Amount)})
}

func (tempHPRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectTempHPs(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.TempHP{Amount: int(r.Amount)}
	}
	return nil
}
