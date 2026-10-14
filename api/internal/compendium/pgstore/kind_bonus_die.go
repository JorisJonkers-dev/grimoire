package pgstore

import (
	"context"
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// bonusDieRows stores bonus dice in compendium.effect_bonus_dice.
type bonusDieRows struct{}

func (bonusDieRows) kind() string { return effects.BonusDie{}.Kind() }

func (bonusDieRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	b, _ := p.(effects.BonusDie)
	return q.InsertEffectBonusDie(ctx, queries.InsertEffectBonusDieParams{
		EffectID: at.effect, Ordinal: at.ordinal, Dice: b.Dice,
		OnAttacks: slices.Contains(b.On, effects.AttackRolls), OnSaves: slices.Contains(b.On, effects.SavingThrows),
	})
}

func (bonusDieRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectBonusDice(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		var on []effects.Roll
		if r.OnAttacks {
			on = append(on, effects.AttackRolls)
		}
		if r.OnSaves {
			on = append(on, effects.SavingThrows)
		}
		into[slot{r.EffectID, r.Ordinal}] = effects.BonusDie{On: on, Dice: r.Dice}
	}
	return nil
}
