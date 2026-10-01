package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

//nolint:gosec // durations and levels are bounded by checks
func saveEffect(ctx context.Context, q *queries.Queries, owner effects.Owner, d effects.Definition) error {
	p := queries.UpsertEffectDefinitionParams{
		Slug: d.Slug, Name: d.Name, Concentration: d.Concentration, OwnerKind: string(owner.Kind), OwnerSlug: owner.Slug,
		DurationKind: pgText(string(d.Duration.Kind)), DurationAmount: int32(d.Duration.Amount), RepeatSave: pgText(d.Duration.RepeatSave),
	}
	id, err := q.UpsertEffectDefinition(ctx, p)
	if err != nil {
		return err
	}
	if err := q.ClearEffectComponents(ctx, id); err != nil {
		return err
	}
	next := int32(0)
	if err := saveComponents(ctx, q, id, &next, pgtype.Int4{}, d.Components); err != nil {
		return err
	}
	return saveScaling(ctx, q, id, d.Scaling)
}

//nolint:gosec // levels are bounded by checks
func saveScaling(ctx context.Context, q *queries.Queries, id int64, sc *effects.Scaling) error {
	if err := q.ClearEffectScaling(ctx, id); err != nil || sc == nil {
		return err
	}
	p := queries.InsertEffectScalingParams{EffectID: id, Axis: string(sc.Axis), ClassSlug: sc.Class, ColumnName: sc.Column, BaseLevel: int32(sc.Base), Dice: sc.Dice}
	if err := q.InsertEffectScaling(ctx, p); err != nil {
		return err
	}
	for _, st := range sc.Steps {
		if err := q.InsertEffectScalingStep(ctx, queries.InsertEffectScalingStepParams{EffectID: id, AtLevel: int32(st.At), Dice: st.Dice}); err != nil {
			return err
		}
	}
	return nil
}

// saveComponents writes components depth first, numbering every one in the Effect; a mode's or a
// branch's children name it as their parent.
func saveComponents(ctx context.Context, q *queries.Queries, id int64, next *int32, parent pgtype.Int4, cs []effects.Component) error {
	for _, c := range cs {
		ord := *next
		*next++
		if err := saveComponent(ctx, q, id, ord, parent, c); err != nil {
			return err
		}
		if err := saveChildren(ctx, q, id, next, ord, c); err != nil {
			return err
		}
	}
	return nil
}

// saveChildren writes a Choice's modes and a Branch's components under them.
func saveChildren(ctx context.Context, q *queries.Queries, id int64, next *int32, ord int32, c effects.Component) error {
	under := pgtype.Int4{Int32: ord, Valid: true}
	switch c := c.(type) {
	case effects.Choice:
		for _, m := range c.Modes {
			mode := *next
			*next++
			if err := q.InsertEffectComponent(ctx, queries.InsertEffectComponentParams{EffectID: id, Ordinal: mode, Kind: "mode", Parent: under}); err != nil {
				return err
			}
			if err := q.InsertEffectMode(ctx, queries.InsertEffectModeParams{EffectID: id, Ordinal: mode, Name: m.Name}); err != nil {
				return err
			}
			if err := saveComponents(ctx, q, id, next, pgtype.Int4{Int32: mode, Valid: true}, m.Components); err != nil {
				return err
			}
		}
	case effects.Branch:
		return saveComponents(ctx, q, id, next, under, c.Then)
	default:
	}
	return nil
}

func saveComponent(ctx context.Context, q *queries.Queries, id int64, ord int32, parent pgtype.Int4, c effects.Component) error {
	kind, insert := componentRow(ctx, q, id, ord, c)
	if err := q.InsertEffectComponent(ctx, queries.InsertEffectComponentParams{EffectID: id, Ordinal: ord, Kind: kind, Parent: parent}); err != nil {
		return err
	}
	return insert()
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
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
	case effects.Reacts:
		return "reacts", func() error {
			return q.InsertEffectReaction(ctx, queries.InsertEffectReactionParams{EffectID: id, Ordinal: ord, Trigger: c.Trigger, Instruction: c.Instruction})
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
	case effects.TempHP, effects.Teleport, effects.ForcedMove, effects.Dispel, effects.Counter, effects.GrantFeature, effects.ResourceChange:
		return widerRow(ctx, q, id, ord, c)
	case effects.Choice:
		return "choice", func() error { return nil }
	case effects.Branch:
		return "branch", func() error {
			return q.InsertEffectBranch(ctx, queries.InsertEffectBranchParams{
				EffectID: id, Ordinal: ord, Condition: string(c.When.Kind), N: int32(c.When.N), CreatureType: c.When.Type, //nolint:gosec // bounded by a check
			})
		}
	}
	return "", func() error { return nil }
}

// widerRow names and inserts the components that move, protect, dispel, counter and grant.
//
//nolint:gosec // every number is bounded by a check
func widerRow(ctx context.Context, q *queries.Queries, id int64, ord int32, c effects.Component) (string, func() error) {
	switch c := c.(type) {
	case effects.TempHP:
		return "temp_hp", func() error {
			return q.InsertEffectTempHP(ctx, queries.InsertEffectTempHPParams{EffectID: id, Ordinal: ord, Amount: int32(c.Amount)})
		}
	case effects.Teleport:
		return "teleport", func() error {
			return q.InsertEffectTeleport(ctx, queries.InsertEffectTeleportParams{EffectID: id, Ordinal: ord, RangeFt: int32(c.RangeFt)})
		}
	case effects.ForcedMove:
		return "forced_move", func() error {
			return q.InsertEffectForcedMove(ctx, queries.InsertEffectForcedMoveParams{EffectID: id, Ordinal: ord, Ft: int32(c.Ft), Toward: c.Toward})
		}
	case effects.Dispel:
		return "dispel", func() error { return nil }
	case effects.Counter:
		return "counter", func() error {
			return q.InsertEffectCounter(ctx, queries.InsertEffectCounterParams{EffectID: id, Ordinal: ord, RangeFt: int32(c.RangeFt)})
		}
	case effects.GrantFeature:
		return "grant_feature", func() error {
			return q.InsertEffectGrant(ctx, queries.InsertEffectGrantParams{EffectID: id, Ordinal: ord, Name: c.Name})
		}
	case effects.ResourceChange:
		return "resource_change", func() error {
			return q.InsertEffectResourceChange(ctx, queries.InsertEffectResourceChangeParams{EffectID: id, Ordinal: ord, ResourceSlug: c.Resource, Delta: int32(c.Delta)})
		}
	case effects.BonusDie, effects.Edge, effects.ExtraDamage, effects.MoveCost, effects.Manual, effects.Area, effects.SaveDamage, effects.SaveCondition,
		effects.CreateSurface, effects.Incapacitated, effects.Immobile, effects.SaveEdge, effects.CritWithin, effects.Exhausting, effects.SpeedPenalty, effects.Reacts,
		effects.Choice, effects.Branch:
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
	scalings, err := s.scalings(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	modes, err := s.q.ListEffectModes(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	names := map[slot]string{}
	for _, m := range modes {
		names[slot{m.EffectID, m.Ordinal}] = m.Name
	}
	byEffect := map[int64][]queries.CompendiumEffectComponent{}
	for _, o := range order {
		byEffect[o.EffectID] = append(byEffect[o.EffectID], o)
	}
	out := effects.Catalog{}
	for _, d := range defs {
		def := effects.Definition{
			Slug: d.Slug, Name: d.Name, Owner: effects.OwnerKind(d.OwnerKind), Concentration: d.Concentration,
			Duration:   effects.Duration{Kind: effects.DurationKind(d.DurationKind.String), Amount: int(d.DurationAmount), RepeatSave: d.RepeatSave.String},
			Scaling:    scalings[d.ID],
			Components: tree(byEffect[d.ID], parts, names),
		}
		out[d.Slug] = def
	}
	return out, nil
}

// tree assembles an Effect's components from their rows, nesting each mode's and branch's children.
func tree(rows []queries.CompendiumEffectComponent, parts map[slot]effects.Component, names map[slot]string) []effects.Component {
	children := map[int32][]queries.CompendiumEffectComponent{}
	var top []queries.CompendiumEffectComponent
	for _, o := range rows {
		if o.Parent.Valid {
			children[o.Parent.Int32] = append(children[o.Parent.Int32], o)
			continue
		}
		top = append(top, o)
	}
	var build func(rows []queries.CompendiumEffectComponent) []effects.Component
	build = func(rows []queries.CompendiumEffectComponent) []effects.Component {
		var out []effects.Component
		for _, o := range rows {
			c := componentOf(o.Kind, parts[slot{o.EffectID, o.Ordinal}])
			switch c := c.(type) {
			case effects.Choice:
				for _, m := range children[o.Ordinal] {
					c.Modes = append(c.Modes, effects.Mode{Name: names[slot{m.EffectID, m.Ordinal}], Components: build(children[m.Ordinal])})
				}
				out = append(out, c)
			case effects.Branch:
				c.Then = build(children[o.Ordinal])
				out = append(out, c)
			default:
				out = append(out, c)
			}
		}
		return out
	}
	return build(top)
}

// scalings reads every Effect's Scaling with its steps.
func (s *Store) scalings(ctx context.Context) (map[int64]*effects.Scaling, error) {
	rows, err := s.q.ListEffectScalings(ctx)
	if err != nil {
		return nil, err
	}
	steps, err := s.q.ListEffectScalingSteps(ctx)
	if err != nil {
		return nil, err
	}
	out := map[int64]*effects.Scaling{}
	for _, r := range rows {
		sc := &effects.Scaling{Axis: effects.Axis(r.Axis), Class: r.ClassSlug, Column: r.ColumnName, Base: int(r.BaseLevel), Dice: r.Dice}
		for _, st := range steps {
			if st.EffectID == r.EffectID {
				sc.Steps = append(sc.Steps, effects.Step{At: int(st.AtLevel), Dice: st.Dice})
			}
		}
		out[r.EffectID] = sc
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
	case "dispel":
		return effects.Dispel{}
	case "choice":
		return effects.Choice{}
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
	reacts, err := s.q.ListEffectReactions(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range reacts {
		out[slot{r.EffectID, r.Ordinal}] = effects.Reacts{Trigger: r.Trigger, Instruction: r.Instruction}
	}
	return s.widerComponents(ctx, out)
}

// widerComponents reads the components that move, protect, counter and grant.
func (s *Store) widerComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	temp, err := s.q.ListEffectTempHPs(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range temp {
		out[slot{r.EffectID, r.Ordinal}] = effects.TempHP{Amount: int(r.Amount)}
	}
	jumps, err := s.q.ListEffectTeleports(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range jumps {
		out[slot{r.EffectID, r.Ordinal}] = effects.Teleport{RangeFt: int(r.RangeFt)}
	}
	pushes, err := s.q.ListEffectForcedMoves(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range pushes {
		out[slot{r.EffectID, r.Ordinal}] = effects.ForcedMove{Ft: int(r.Ft), Toward: r.Toward}
	}
	counters, err := s.q.ListEffectCounters(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range counters {
		out[slot{r.EffectID, r.Ordinal}] = effects.Counter{RangeFt: int(r.RangeFt)}
	}
	grants, err := s.q.ListEffectGrants(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range grants {
		out[slot{r.EffectID, r.Ordinal}] = effects.GrantFeature{Name: r.Name}
	}
	changes, err := s.q.ListEffectResourceChanges(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range changes {
		out[slot{r.EffectID, r.Ordinal}] = effects.ResourceChange{Resource: r.ResourceSlug, Delta: int(r.Delta)}
	}
	return s.shapeComponents(ctx, out)
}

// shapeComponents reads the branches that give components their structure.
func (s *Store) shapeComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	branches, err := s.q.ListEffectBranches(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range branches {
		out[slot{r.EffectID, r.Ordinal}] = effects.Branch{When: effects.Condition{Kind: effects.ConditionKind(r.Condition), N: int(r.N), Type: r.CreatureType}, Then: nil}
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
