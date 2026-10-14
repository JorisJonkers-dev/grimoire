package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// summonRows stores summons in compendium.effect_summons.
type summonRows struct{}

func (summonRows) kind() string { return effects.Summon{}.Kind() }

//nolint:gosec // the count is bounded by a check
func (summonRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	s, _ := p.(effects.Summon)
	return q.InsertEffectSummon(ctx, queries.InsertEffectSummonParams{
		EffectID: at.effect, Ordinal: at.ordinal, MonsterSlug: s.Monster, Count: int32(s.Count), SharesTurn: s.Shares, NeedsCommand: s.NeedsCommand,
	})
}

func (summonRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectSummons(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Summon{Monster: r.MonsterSlug, Count: int(r.Count), Shares: r.SharesTurn, NeedsCommand: r.NeedsCommand}
	}
	return nil
}
