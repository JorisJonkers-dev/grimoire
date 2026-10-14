package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// saveDamageRows stores damage saved against in compendium.effect_save_damage.
type saveDamageRows struct{}

func (saveDamageRows) kind() string { return effects.SaveDamage{}.Kind() }

func (saveDamageRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.SaveDamage)
	return q.InsertEffectSaveDamage(ctx, queries.InsertEffectSaveDamageParams{
		EffectID: at.effect, Ordinal: at.ordinal, Ability: s.Ability, Dice: s.Dice, DamageType: s.Type, Half: s.Half,
	})
}

func (saveDamageRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectSaveDamage(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.SaveDamage{Ability: r.Ability, Dice: r.Dice, Type: r.DamageType, Half: r.Half}
	}
	return nil
}
