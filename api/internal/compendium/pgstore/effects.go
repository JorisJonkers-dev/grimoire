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

// saveChildren writes a Choice's modes, and the Parts beneath a Part that holds any, under them.
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
	case effects.Part:
		if h, ok := holdingOf(c.Kind()); ok {
			return saveComponents(ctx, q, id, next, under, h.beneath(c))
		}
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
	case effects.Part:
		return c.Kind(), func() error {
			rows, err := rowsOf(c.Kind())
			if err != nil {
				return err
			}
			return rows.insert(ctx, q, slot{id, ord}, c)
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
	case effects.SaveCondition:
		return "save_condition", func() error {
			return q.InsertEffectSaveCondition(ctx, queries.InsertEffectSaveConditionParams{EffectID: id, Ordinal: ord, Ability: c.Ability, ConditionSlug: c.Slug})
		}
	case effects.CreateSurface:
		return "create_surface", func() error {
			return q.InsertEffectSurface(ctx, queries.InsertEffectSurfaceParams{EffectID: id, Ordinal: ord, Surface: string(c.Kind), Rounds: int32(c.Rounds)}) //nolint:gosec // bounded by a check
		}
	case effects.ForcedMove:
		return "forced_move", func() error {
			return q.InsertEffectForcedMove(ctx, queries.InsertEffectForcedMoveParams{EffectID: id, Ordinal: ord, Ft: int32(c.Ft), Toward: c.Toward}) //nolint:gosec // bounded by a check
		}
	case effects.Choice:
		return "choice", func() error { return nil }
	case effects.Reveal:
		return "reveal", func() error {
			return q.InsertEffectReveal(ctx, queries.InsertEffectRevealParams{EffectID: id, Ordinal: ord, Qualities: c.Qualities})
		}
	case effects.Form:
		return "form", func() error {
			return q.InsertEffectForm(ctx, queries.InsertEffectFormParams{EffectID: id, Ordinal: ord, MonsterSlug: c.Monster, TempHp: int32(c.TempHP)}) //nolint:gosec // bounded by a check
		}
	case effects.Summon:
		return "summon", func() error {
			return q.InsertEffectSummon(ctx, queries.InsertEffectSummonParams{
				EffectID: id, Ordinal: ord, MonsterSlug: c.Monster, Count: int32(c.Count), SharesTurn: c.Shares, NeedsCommand: c.NeedsCommand, //nolint:gosec // bounded by a check
			})
		}
	case effects.AreaSave, effects.Light:
		// The Effect builder's parts live in a spell's design, never in the catalogue's tables.
	}
	return "", func() error { return nil }
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

// tree assembles an Effect's components from their rows, nesting the children of each mode and of each
// Part that holds Parts beneath it.
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
			case effects.Part:
				out = append(out, nested(c, build(children[o.Ordinal])))
			default:
				out = append(out, c)
			}
		}
		return out
	}
	return build(top)
}

// nested puts the Parts read beneath a Part back under it, when its kind holds any.
func nested(p effects.Part, beneath []effects.Component) effects.Component {
	if h, ok := holdingOf(p.Kind()); ok {
		return h.nest(p, beneath)
	}
	return p
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
	if kind == "choice" {
		return effects.Choice{}
	}
	return typed
}

// componentsBySlot reads every typed component row.
func (s *Store) componentsBySlot(ctx context.Context) (map[slot]effects.Component, error) {
	out := map[slot]effects.Component{}
	if err := readParts(ctx, s.q, out); err != nil {
		return nil, err
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

// areaComponents reads the components that shape and resolve an area, and push creatures out of it.
func (s *Store) areaComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	areas, err := s.q.ListEffectAreas(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range areas {
		out[slot{r.EffectID, r.Ordinal}] = effects.Area{Shape: hex.Shape(r.Shape), SizeFt: int(r.SizeFt), RangeFt: int(r.RangeFt)}
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
	pushes, err := s.q.ListEffectForcedMoves(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range pushes {
		out[slot{r.EffectID, r.Ordinal}] = effects.ForcedMove{Ft: int(r.Ft), Toward: r.Toward}
	}
	return s.shapeComponents(ctx, out)
}

// shapeComponents reads the components that summon and transform.
func (s *Store) shapeComponents(ctx context.Context, out map[slot]effects.Component) (map[slot]effects.Component, error) {
	summons, err := s.q.ListEffectSummons(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range summons {
		out[slot{r.EffectID, r.Ordinal}] = effects.Summon{Monster: r.MonsterSlug, Count: int(r.Count), Shares: r.SharesTurn, NeedsCommand: r.NeedsCommand}
	}
	forms, err := s.q.ListEffectForms(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range forms {
		out[slot{r.EffectID, r.Ordinal}] = effects.Form{Monster: r.MonsterSlug, TempHP: int(r.TempHp)}
	}
	reveals, err := s.q.ListEffectReveals(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range reveals {
		out[slot{r.EffectID, r.Ordinal}] = effects.Reveal{Qualities: r.Qualities}
	}
	return out, nil
}
