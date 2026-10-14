package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// teleportRows stores teleports in compendium.effect_teleports.
type teleportRows struct{}

func (teleportRows) kind() string { return effects.Teleport{}.Kind() }

//nolint:gosec // the range is bounded by a check
func (teleportRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	t, _ := p.(effects.Teleport)
	return q.InsertEffectTeleport(ctx, queries.InsertEffectTeleportParams{EffectID: at.effect, Ordinal: at.ordinal, RangeFt: int32(t.RangeFt)})
}

func (teleportRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectTeleports(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Teleport{RangeFt: int(r.RangeFt)}
	}
	return nil
}
