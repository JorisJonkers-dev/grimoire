package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// saveScaling replaces an Effect's Scaling, one row per rule in order, with the steps of each table typed
// as the field it grows.
//
//nolint:gosec // positions, levels and parts are bounded by checks
func saveScaling(ctx context.Context, q *queries.Queries, id int64, d effects.Definition) error {
	if err := q.ClearEffectScalingRules(ctx, id); err != nil {
		return err
	}
	for i, s := range d.Scaling {
		f, ok := d.Scaled(s)
		if !ok {
			return fmt.Errorf("scaling %d grows %s, which nothing there describes", i+1, s.Field)
		}
		p := queries.InsertEffectScalingRuleParams{
			EffectID: id, Position: int32(i), Part: pgtype.Int4{Int32: int32(s.Part), Valid: s.Part != effects.OnActivation}, Field: s.Field,
			Axis: string(s.Axis), ClassSlug: s.Class, BaseLevel: 0, PerDice: pgtype.Text{}, PerNumber: pgtype.Int4{}, Formula: pgtype.Text{}, ColumnName: "",
		}
		var steps []effects.Step
		switch r := s.Rule.(type) {
		case effects.Linear:
			p.Rule, p.BaseLevel = "linear", int32(r.Base)
			p.PerDice, p.PerNumber = typed(f, r.Per).dice, typed(f, r.Per).number
		case effects.Table:
			p.Rule, steps = "table", r.Steps
		case effects.Formula:
			p.Rule, p.Formula = "formula", pgText(r.Expr)
		case effects.Column:
			p.Rule, p.ColumnName = "column", r.Name
		}
		if err := q.InsertEffectScalingRule(ctx, p); err != nil {
			return err
		}
		for _, st := range steps {
			v := typed(f, st.Value)
			sp := queries.InsertEffectScalingRuleStepParams{EffectID: id, Position: int32(i), AtLevel: int32(st.At), ValueDice: v.dice, ValueNumber: v.number, ValueFlag: v.flag, ValueText: v.text}
			if err := q.InsertEffectScalingRuleStep(ctx, sp); err != nil {
				return err
			}
		}
	}
	return nil
}

// column is a value in the one column its field's type keeps it in.
type column struct {
	dice   pgtype.Text
	number pgtype.Int4
	flag   pgtype.Bool
	text   pgtype.Text
}

//nolint:gosec // numbers are bounded by checks
func typed(f effects.Field, v effects.Value) column {
	var out column
	switch f.Type {
	case effects.DiceField:
		out.dice = pgtype.Text{String: v.Text, Valid: true}
	case effects.NumberField:
		out.number = pgtype.Int4{Int32: int32(v.Number), Valid: true}
	case effects.FlagField:
		out.flag = pgtype.Bool{Bool: v.Flag, Valid: true}
	case effects.TextField, effects.ChoiceField, effects.ChoicesField:
		out.text = pgtype.Text{String: v.Text, Valid: true}
	}
	return out
}

// scalings reads every Effect's Scaling, rule by rule in order, with the steps of each table.
func (s *Store) scalings(ctx context.Context) (map[int64][]effects.Scaling, error) {
	rows, err := s.q.ListEffectScalingRules(ctx)
	if err != nil {
		return nil, err
	}
	steps, err := s.q.ListEffectScalingRuleSteps(ctx)
	if err != nil {
		return nil, err
	}
	type key struct {
		effect   int64
		position int32
	}
	byRule := map[key][]effects.Step{}
	for _, st := range steps {
		v := effects.Value{Text: st.ValueDice.String + st.ValueText.String, Number: int(st.ValueNumber.Int32), Flag: st.ValueFlag.Bool, Texts: nil}
		byRule[key{st.EffectID, st.Position}] = append(byRule[key{st.EffectID, st.Position}], effects.Step{At: int(st.AtLevel), Value: v})
	}
	out := map[int64][]effects.Scaling{}
	for _, r := range rows {
		part := effects.OnActivation
		if r.Part.Valid {
			part = int(r.Part.Int32)
		}
		sc := effects.Scaling{Part: part, Field: r.Field, Axis: effects.Axis(r.Axis), Class: r.ClassSlug, Rule: rule(r, byRule[key{r.EffectID, r.Position}])}
		out[r.EffectID] = append(out[r.EffectID], sc)
	}
	return out, nil
}

// rule is a stored rule as the rules engine reads it.
func rule(r queries.CompendiumEffectScalingRule, steps []effects.Step) effects.Rule {
	switch r.Rule {
	case "linear":
		return effects.Linear{Base: int(r.BaseLevel), Per: effects.Value{Text: r.PerDice.String, Number: int(r.PerNumber.Int32), Flag: false, Texts: nil}}
	case "formula":
		return effects.Formula{Expr: r.Formula.String}
	case "column":
		return effects.Column{Name: r.ColumnName}
	}
	return effects.Table{Steps: steps}
}
