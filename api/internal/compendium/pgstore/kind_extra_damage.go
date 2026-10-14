package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// extraDamageRows stores extra damage in compendium.effect_extra_damage.
type extraDamageRows struct{}

func (extraDamageRows) kind() string { return effects.ExtraDamage{}.Kind() }

func (extraDamageRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	x, _ := p.(effects.ExtraDamage)
	return q.InsertEffectExtraDamage(ctx, queries.InsertEffectExtraDamageParams{EffectID: at.effect, Ordinal: at.ordinal, Dice: x.Dice})
}

func (extraDamageRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectExtraDamage(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.ExtraDamage{Dice: r.Dice}
	}
	return nil
}
