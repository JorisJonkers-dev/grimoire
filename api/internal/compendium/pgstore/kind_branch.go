package pgstore

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// branchRows stores branches in compendium.effect_branches; the Parts a branch adds are component
// rows that name it as their parent, in its group then.
type branchRows struct{}

func (branchRows) kind() string { return effects.Branch{}.Kind() }

//nolint:gosec // n is bounded by a check
func (branchRows) insert(ctx context.Context, q *queries.Queries, at slot, p effects.Part) error {
	b, _ := p.(effects.Branch)
	return q.InsertEffectBranch(ctx, queries.InsertEffectBranchParams{
		EffectID: at.effect, Ordinal: at.ordinal, Condition: string(b.When.Kind), N: int32(b.When.N), CreatureType: b.When.Type,
	})
}

func (branchRows) read(ctx context.Context, q *queries.Queries, into map[slot]effects.Component) error {
	rows, err := q.ListEffectBranches(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		into[slot{r.EffectID, r.Ordinal}] = effects.Branch{When: effects.Condition{Kind: effects.ConditionKind(r.Condition), N: int(r.N), Type: r.CreatureType}, Then: nil}
	}
	return nil
}

func (branchRows) nest(p effects.Part, groups []effects.Group) effects.Part {
	b, _ := p.(effects.Branch)
	b.Then = effects.Groups(groups).Of("then")
	return b
}
