package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// formRows stores forms in compendium.effect_forms.
type formRows struct{}

func (formRows) kind() string { return effects.Form{}.Kind() }

//nolint:gosec // hit points are bounded by a check
func (formRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	f, _ := p.(effects.Form)
	return q.InsertEffectForm(ctx, queries.InsertEffectFormParams{EffectID: at.effect, Ordinal: at.ordinal, MonsterSlug: f.Monster, TempHp: int32(f.TempHP)})
}

func (formRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Part) error {
	rows, err := q.ListEffectForms(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Form{Monster: r.MonsterSlug, TempHP: int(r.TempHp)}
	}
	return nil
}
