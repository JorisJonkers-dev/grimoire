package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// reachName stores a Range by name.
func reachName(r effects.Range) string {
	switch r {
	case effects.WithinFive:
		return "within_five"
	case effects.BeyondFive:
		return "beyond_five"
	case effects.AnyRange:
	}
	return "any"
}

// SaveEffect stores an Effect and replaces its components, in one transaction.
func (s *Store) SaveEffect(ctx context.Context, owner effects.Owner, d effects.Definition) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return saveEffect(ctx, queries.New(tx), owner, d)
	})
	if err != nil {
		return fmt.Errorf("compendium: save effect %s: %w", d.Slug, err)
	}
	return nil
}

func saveEffect(ctx context.Context, q *queries.Queries, owner effects.Owner, d effects.Definition) error {
	id, err := q.UpsertEffectDefinition(ctx, queries.UpsertEffectDefinitionParams{
		Slug: d.Slug, Name: d.Name, Concentration: d.Concentration, OwnerKind: string(owner.Kind), OwnerSlug: owner.Slug,
	})
	if err != nil {
		return err
	}
	if err := q.ClearEffectComponents(ctx, id); err != nil {
		return err
	}
	for i, c := range d.Components {
		if err := saveComponent(ctx, q, id, int32(i), c); err != nil { //nolint:gosec // a handful of components
			return err
		}
	}
	return nil
}

func saveComponent(ctx context.Context, q *queries.Queries, id int64, ord int32, c effects.Component) error {
	kind, insert := componentRow(ctx, q, id, ord, c)
	if err := q.InsertEffectComponent(ctx, queries.InsertEffectComponentParams{EffectID: id, Ordinal: ord, Kind: kind}); err != nil {
		return err
	}
	return insert()
}

// componentRow names a component's kind and returns the insert for its typed row.
func componentRow(ctx context.Context, q *queries.Queries, id int64, ord int32, c effects.Component) (string, func() error) {
	switch c := c.(type) {
	case effects.BonusDie:
		return "bonus_die", func() error {
			return q.InsertEffectBonusDie(ctx, queries.InsertEffectBonusDieParams{
				EffectID: id, Ordinal: ord, Dice: c.Dice, OnAttacks: has(c.On, effects.AttackRolls), OnSaves: has(c.On, effects.SavingThrows),
			})
		}
	case effects.Edge:
		return "edge", func() error {
			return q.InsertEffectEdge(ctx, queries.InsertEffectEdgeParams{
				EffectID: id, Ordinal: ord, Against: c.Against, Advantage: c.Advantage, Reach: reachName(c.Range), SourceOnly: c.SourceOnly,
			})
		}
	case effects.ExtraDamage:
		return "extra_damage", func() error {
			return q.InsertEffectExtraDamage(ctx, queries.InsertEffectExtraDamageParams{EffectID: id, Ordinal: ord, Dice: c.Dice})
		}
	case effects.MoveCost:
		return "move_cost", func() error {
			return q.InsertEffectMoveCost(ctx, queries.InsertEffectMoveCostParams{EffectID: id, Ordinal: ord, Multiplier: int32(c.Multiplier)}) //nolint:gosec // bounded by a check
		}
	case effects.Manual:
		return "manual", func() error {
			return q.InsertEffectManual(ctx, queries.InsertEffectManualParams{EffectID: id, Ordinal: ord, Instruction: c.Instruction})
		}
	case effects.Area:
		return "area", func() error {
			return q.InsertEffectArea(ctx, queries.InsertEffectAreaParams{
				EffectID: id, Ordinal: ord, Shape: string(c.Shape), SizeFt: int32(c.SizeFt), RangeFt: int32(c.RangeFt), //nolint:gosec // bounded by checks
			})
		}
	case effects.SaveDamage:
		return "save_damage", func() error {
			return q.InsertEffectSaveDamage(ctx, queries.InsertEffectSaveDamageParams{
				EffectID: id, Ordinal: ord, Ability: c.Ability, Dice: c.Dice, DamageType: c.Type, Half: c.Half,
			})
		}
	case effects.SaveCondition:
		return "save_condition", func() error {
			return q.InsertEffectSaveCondition(ctx, queries.InsertEffectSaveConditionParams{EffectID: id, Ordinal: ord, Ability: c.Ability, ConditionSlug: c.Slug})
		}
	case effects.CreateSurface:
		return "create_surface", func() error {
			return q.InsertEffectSurface(ctx, queries.InsertEffectSurfaceParams{EffectID: id, Ordinal: ord, Surface: string(c.Kind), Rounds: int32(c.Rounds)}) //nolint:gosec // bounded by a check
		}
	case effects.SpeedPenalty:
		return "speed_penalty", func() error {
			return q.InsertEffectSpeedPenalty(ctx, queries.InsertEffectSpeedPenaltyParams{EffectID: id, Ordinal: ord, Ft: int32(c.Ft)}) //nolint:gosec // bounded by a check
		}
	case effects.Incapacitated:
		return "incapacitated", func() error { return nil }
	case effects.Immobile:
		return "immobile", func() error { return nil }
	case effects.SaveEdge:
		return "save_edge", func() error {
			return q.InsertEffectSaveEdge(ctx, queries.InsertEffectSaveEdgeParams{EffectID: id, Ordinal: ord, Ability: c.Ability, Mode: string(c.Mode)})
		}
	case effects.CritWithin:
		return "crit_within", func() error {
			return q.InsertEffectCrit(ctx, queries.InsertEffectCritParams{EffectID: id, Ordinal: ord, Feet: int32(c.Feet)}) //nolint:gosec // bounded by a check
		}
	case effects.Exhausting:
		return "exhausting", func() error {
			return q.InsertEffectExhaustion(ctx, queries.InsertEffectExhaustionParams{
				EffectID: id, Ordinal: ord, D20PerLevel: int32(c.D20PerLevel), SpeedFtPerLevel: int32(c.SpeedFtPerLevel), DeathAt: int32(c.DeathAt), //nolint:gosec // bounded by checks
			})
		}
	}
	return "", func() error { return nil }
}

func has(rolls []effects.Roll, r effects.Roll) bool {
	for _, x := range rolls {
		if x == r {
			return true
		}
	}
	return false
}

// slot is one component's place in an Effect.
type slot struct {
	effect  int64
	ordinal int32
}

// Effects reads every stored Effect into a Catalog: one query per table, assembled in memory.
func (s *Store) Effects(ctx context.Context) (effects.Catalog, error) {
	parts, err := s.componentsBySlot(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	defs, err := s.q.ListEffectDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	order, err := s.q.ListEffectComponents(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	byID := map[int64]*effects.Definition{}
	out := effects.Catalog{}
	for _, d := range defs {
		byID[d.ID] = &effects.Definition{Slug: d.Slug, Name: d.Name, Concentration: d.Concentration}
	}
	for _, o := range order {
		d := byID[o.EffectID]
		d.Components = append(d.Components, componentOf(o.Kind, parts[slot{o.EffectID, o.Ordinal}]))
	}
	for _, d := range byID {
		out[d.Slug] = *d
	}
	return out, nil
}

// componentOf is a component read from its typed row, or made from its kind when it carries no data.
func componentOf(kind string, typed effects.Component) effects.Component {
	switch kind {
	case "incapacitated":
		return effects.Incapacitated{}
	case "immobile":
		return effects.Immobile{}
	}
	return typed
}

// componentsBySlot reads every typed component row.
func (s *Store) componentsBySlot(ctx context.Context) (map[slot]effects.Component, error) {
	out := map[slot]effects.Component{}
	bonus, err := s.q.ListEffectBonusDice(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range bonus {
		var on []effects.Roll
		if r.OnAttacks {
			on = append(on, effects.AttackRolls)
		}
		if r.OnSaves {
			on = append(on, effects.SavingThrows)
		}
		out[slot{r.EffectID, r.Ordinal}] = effects.BonusDie{On: on, Dice: r.Dice}
	}
	edges, err := s.q.ListEffectEdges(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range edges {
		out[slot{r.EffectID, r.Ordinal}] = effects.Edge{Against: r.Against, Advantage: r.Advantage, Range: reachOf(r.Reach), SourceOnly: r.SourceOnly}
	}
	extra, err := s.q.ListEffectExtraDamage(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range extra {
		out[slot{r.EffectID, r.Ordinal}] = effects.ExtraDamage{Dice: r.Dice}
	}
	moves, err := s.q.ListEffectMoveCosts(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range moves {
		out[slot{r.EffectID, r.Ordinal}] = effects.MoveCost{Multiplier: int(r.Multiplier)}
	}
	manual, err := s.q.ListEffectManual(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range manual {
		out[slot{r.EffectID, r.Ordinal}] = effects.Manual{Instruction: r.Instruction}
	}
	return s.areaComponents(ctx, out)
}

// areaComponents reads the components that shape and resolve an area.
func (s *Store) areaComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	areas, err := s.q.ListEffectAreas(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range areas {
		out[slot{r.EffectID, r.Ordinal}] = effects.Area{Shape: hex.Shape(r.Shape), SizeFt: int(r.SizeFt), RangeFt: int(r.RangeFt)}
	}
	damage, err := s.q.ListEffectSaveDamage(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range damage {
		out[slot{r.EffectID, r.Ordinal}] = effects.SaveDamage{Ability: r.Ability, Dice: r.Dice, Type: r.DamageType, Half: r.Half}
	}
	conds, err := s.q.ListEffectSaveConditions(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range conds {
		out[slot{r.EffectID, r.Ordinal}] = effects.SaveCondition{Ability: r.Ability, Slug: r.ConditionSlug}
	}
	surfaces, err := s.q.ListEffectSurfaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range surfaces {
		out[slot{r.EffectID, r.Ordinal}] = effects.CreateSurface{Kind: surface.Kind(r.Surface), Rounds: int(r.Rounds)}
	}
	return s.conditionComponents(ctx, out)
}

// conditionComponents reads the components conditions are made of.
func (s *Store) conditionComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	edges, err := s.q.ListEffectSaveEdges(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range edges {
		out[slot{r.EffectID, r.Ordinal}] = effects.SaveEdge{Ability: r.Ability, Mode: effects.SaveMode(r.Mode)}
	}
	crits, err := s.q.ListEffectCrits(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range crits {
		out[slot{r.EffectID, r.Ordinal}] = effects.CritWithin{Feet: int(r.Feet)}
	}
	tired, err := s.q.ListEffectExhaustion(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range tired {
		out[slot{r.EffectID, r.Ordinal}] = effects.Exhausting{D20PerLevel: int(r.D20PerLevel), SpeedFtPerLevel: int(r.SpeedFtPerLevel), DeathAt: int(r.DeathAt)}
	}
	slow, err := s.q.ListEffectSpeedPenalties(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range slow {
		out[slot{r.EffectID, r.Ordinal}] = effects.SpeedPenalty{Ft: int(r.Ft)}
	}
	return out, nil
}

// reachOf reads a stored reach; the database allows only the three names.
func reachOf(s string) effects.Range {
	switch s {
	case "within_five":
		return effects.WithinFive
	case "beyond_five":
		return effects.BeyondFive
	}
	return effects.AnyRange
}
