package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// SaveEffect stores an Effect and replaces its Parts, in one transaction.
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
	if err := q.ClearEffectParts(ctx, id); err != nil {
		return err
	}
	next := int32(0)
	if err := saveParts(ctx, q, id, &next, pgtype.Int4{}, pgtype.Text{}, d.Parts); err != nil {
		return err
	}
	if err := saveTargeting(ctx, q, id, d.Targeting); err != nil {
		return err
	}
	return saveScaling(ctx, q, id, d)
}

// saveTargeting writes what the Effect's Activation may pick, or clears it when the Effect states none.
//
//nolint:gosec // counts and ranges are bounded by checks
func saveTargeting(ctx context.Context, q *queries.Queries, id int64, t effects.Targeting) error {
	if t.Kind == "" {
		return q.ClearEffectTargeting(ctx, id)
	}
	types := t.Types
	if types == nil {
		types = []string{}
	}
	return q.UpsertEffectTargeting(ctx, queries.UpsertEffectTargetingParams{
		EffectID: id, Kind: string(t.Kind), TargetCount: int32(t.Count), RangeFt: int32(t.RangeFt), Side: string(t.Side),
		CreatureTypes: types, Seen: t.Seen, Willing: t.Willing, SpreadFt: int32(t.SpreadFt),
	})
}

// targetings reads every Effect's Targeting, by Effect.
func (s *Store) targetings(ctx context.Context) (map[int64]effects.Targeting, error) {
	rows, err := s.q.ListEffectTargeting(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]effects.Targeting, len(rows))
	for _, r := range rows {
		var types []string
		if len(r.CreatureTypes) > 0 {
			types = r.CreatureTypes
		}
		out[r.EffectID] = effects.Targeting{
			Kind: effects.TargetKind(r.Kind), Count: int(r.TargetCount), RangeFt: int(r.RangeFt), Side: effects.Side(r.Side),
			Types: types, Seen: r.Seen, Willing: r.Willing, SpreadFt: int(r.SpreadFt),
		}
	}
	return out, nil
}

// saveParts writes Parts depth first, numbering every one in the Effect; the Parts beneath a
// Part name it as their parent, and the group they sit in.
func saveParts(ctx context.Context, q *queries.Queries, id int64, next *int32, parent pgtype.Int4, group pgtype.Text, cs []effects.Part) error {
	for _, c := range cs {
		ord := *next
		*next++
		if err := savePart(ctx, q, id, slot{id, ord}, parent, group, c); err != nil {
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
func saveGroups(ctx context.Context, q *queries.Queries, id int64, next *int32, ord int32, c effects.Part) error {
	g, ok := c.(effects.Grouper)
	if !ok {
		return nil
	}
	for _, group := range g.Groups() {
		if len(group.Parts) == 0 {
			return fmt.Errorf("%s group %q holds no Parts", c.Kind(), group.Name)
		}
		if err := saveParts(ctx, q, id, next, pgtype.Int4{Int32: ord, Valid: true}, pgText(group.Name), group.Parts); err != nil {
			return err
		}
	}
	return nil
}

func savePart(ctx context.Context, q *queries.Queries, id int64, at slot, parent pgtype.Int4, group pgtype.Text, c effects.Part) error {
	rows, err := rowsOf(c.Kind())
	if err != nil {
		return err
	}
	p := queries.InsertEffectPartParams{EffectID: id, Ordinal: at.ordinal, Kind: c.Kind(), Parent: parent, GroupName: group}
	if err := q.InsertEffectPart(ctx, p); err != nil {
		return err
	}
	return rows.insert(ctx, q, at, c)
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// slot is one Part's place in an Effect.
type slot struct {
	effect  int64
	ordinal int32
}

// Effects reads every stored Effect into a Catalog: one query per table, assembled in memory.
func (s *Store) Effects(ctx context.Context) (effects.Catalog, error) {
	parts := map[slot]effects.Part{}
	if err := readParts(ctx, s.q, parts); err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	defs, err := s.q.ListEffectDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	order, err := s.q.ListEffectParts(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	scalings, err := s.scalings(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	targetings, err := s.targetings(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: effects: %w", err)
	}
	byEffect := map[int64][]queries.CompendiumEffectPart{}
	for _, o := range order {
		byEffect[o.EffectID] = append(byEffect[o.EffectID], o)
	}
	out := effects.Catalog{}
	for _, d := range defs {
		def := effects.Definition{
			Slug: d.Slug, Name: d.Name, Owner: effects.OwnerKind(d.OwnerKind), Concentration: d.Concentration,
			Duration:  effects.Duration{Kind: effects.DurationKind(d.DurationKind.String), Amount: int(d.DurationAmount), RepeatSave: d.RepeatSave.String},
			Targeting: targetings[d.ID],
			Scaling:   scalings[d.ID],
			Parts:     tree(byEffect[d.ID], parts),
		}
		out[d.Slug] = def
	}
	return out, nil
}

// tree assembles an Effect's Parts from their rows, nesting the Parts beneath each Part that holds any
// in their named groups.
func tree(rows []queries.CompendiumEffectPart, parts map[slot]effects.Part) []effects.Part {
	children := map[int32][]queries.CompendiumEffectPart{}
	var top []queries.CompendiumEffectPart
	for _, o := range rows {
		if o.Parent.Valid {
			children[o.Parent.Int32] = append(children[o.Parent.Int32], o)
			continue
		}
		top = append(top, o)
	}
	var build func(rows []queries.CompendiumEffectPart) []effects.Part
	build = func(rows []queries.CompendiumEffectPart) []effects.Part {
		var out []effects.Part
		for _, o := range rows {
			out = append(out, nested(o.Kind, parts[slot{o.EffectID, o.Ordinal}], grouped(children[o.Ordinal], build)))
		}
		return out
	}
	return build(top)
}

// grouped puts the rows beneath a Part into their named groups, in the order the groups first come.
func grouped(rows []queries.CompendiumEffectPart, build func([]queries.CompendiumEffectPart) []effects.Part) []effects.Group {
	var names []string
	byName := map[string][]queries.CompendiumEffectPart{}
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
func nested(kind string, p effects.Part, groups []effects.Group) effects.Part {
	if h, ok := holdingOf(kind); ok {
		return h.nest(p, groups)
	}
	return p
}
