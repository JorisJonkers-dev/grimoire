package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
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
	if err := saveComponents(ctx, q, id, &next, pgtype.Int4{}, pgtype.Text{}, d.Components); err != nil {
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

// saveComponents writes Parts depth first, numbering every one in the Effect; the Parts beneath a
// Part name it as their parent, and the group they sit in.
func saveComponents(ctx context.Context, q *queries.Queries, id int64, next *int32, parent pgtype.Int4, group pgtype.Text, cs []effects.Component) error {
	for _, c := range cs {
		ord := *next
		*next++
		if err := saveComponent(ctx, q, id, slot{id, ord}, parent, group, c); err != nil {
			return err
		}
		if err := saveGroups(ctx, q, id, next, ord, c); err != nil {
			return err
		}
	}
	return nil
}

// saveGroups writes the Parts beneath a Part that holds any, group by group. A group of no Parts
// would leave no row to read back, so it is refused.
func saveGroups(ctx context.Context, q *queries.Queries, id int64, next *int32, ord int32, c effects.Component) error {
	g, ok := c.(effects.Grouper)
	if !ok {
		return nil
	}
	for _, group := range g.Groups() {
		if len(group.Parts) == 0 {
			return fmt.Errorf("%s group %q holds no Parts", c.Kind(), group.Name)
		}
		if err := saveComponents(ctx, q, id, next, pgtype.Int4{Int32: ord, Valid: true}, pgText(group.Name), group.Parts); err != nil {
			return err
		}
	}
	return nil
}

func saveComponent(ctx context.Context, q *queries.Queries, id int64, at slot, parent pgtype.Int4, group pgtype.Text, c effects.Component) error {
	rows, err := rowsOf(c.Kind())
	if err != nil {
		return err
	}
	p := queries.InsertEffectComponentParams{EffectID: id, Ordinal: at.ordinal, Kind: c.Kind(), Parent: parent, GroupName: group}
	if err := q.InsertEffectComponent(ctx, p); err != nil {
		return err
	}
	return rows.insert(ctx, q, at, c)
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// slot is one component's place in an Effect.
type slot struct {
	effect  int64
	ordinal int32
}

// Effects reads every stored Effect into a Catalog: one query per table, assembled in memory.
func (s *Store) Effects(ctx context.Context) (effects.Catalog, error) {
	parts := map[slot]effects.Component{}
	if err := readParts(ctx, s.q, parts); err != nil {
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
			Components: tree(byEffect[d.ID], parts),
		}
		out[d.Slug] = def
	}
	return out, nil
}

// tree assembles an Effect's Parts from their rows, nesting the Parts beneath each Part that holds any
// in their named groups.
func tree(rows []queries.CompendiumEffectComponent, parts map[slot]effects.Component) []effects.Component {
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
			out = append(out, nested(o.Kind, parts[slot{o.EffectID, o.Ordinal}], grouped(children[o.Ordinal], build)))
		}
		return out
	}
	return build(top)
}

// grouped puts the rows beneath a Part into their named groups, in the order the groups first come.
func grouped(rows []queries.CompendiumEffectComponent, build func([]queries.CompendiumEffectComponent) []effects.Component) []effects.Group {
	var names []string
	byName := map[string][]queries.CompendiumEffectComponent{}
	for _, o := range rows {
		if _, seen := byName[o.GroupName.String]; !seen {
			names = append(names, o.GroupName.String)
		}
		byName[o.GroupName.String] = append(byName[o.GroupName.String], o)
	}
	out := make([]effects.Group, 0, len(names))
	for _, name := range names {
		out = append(out, effects.Group{Name: name, Parts: build(byName[name])})
	}
	return out
}

// nested puts the Parts read beneath a Part back under it, when its kind holds any.
func nested(kind string, p effects.Component, groups []effects.Group) effects.Component {
	if h, ok := holdingOf(kind); ok {
		return h.nest(p, groups)
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
