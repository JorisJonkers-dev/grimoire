package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// Surfaces reads every Surface definition with how damage changes it.
func (s *Store) Surfaces(ctx context.Context) (surface.Catalog, error) {
	defs, err := s.q.ListSurfaceDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: surfaces: %w", err)
	}
	reactions, err := s.q.ListSurfaceReactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("compendium: surfaces: %w", err)
	}
	out := surface.Catalog{}
	for _, d := range defs {
		out[surface.Kind(d.Slug)] = surface.Definition{
			Kind: surface.Kind(d.Slug), Name: d.Name, Cost: int(d.Cost), Obscures: surface.Obscurement(d.Obscures.String),
			HazardDice: d.HazardDice.String, HazardType: d.HazardType.String, EveryStep: d.EveryStep, Effect: d.EffectSlug.String, Reactions: nil,
		}
	}
	for _, r := range reactions {
		d := out[surface.Kind(r.Surface)]
		d.Reactions = append(d.Reactions, surface.Reaction{Damage: r.DamageType, Becomes: surface.Kind(r.Becomes)})
		out[d.Kind] = d
	}
	return out, nil
}

// SaveSurface stores an author's Surface and replaces how damage changes it, in one transaction.
func (s *Store) SaveSurface(ctx context.Context, d surface.Definition) error {
	text := func(v string) pgtype.Text { return pgtype.Text{String: v, Valid: v != ""} }
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(tx)
		p := queries.UpsertSurfaceDefinitionParams{
			Slug: string(d.Kind), Name: d.Name, Cost: int32(max(1, d.Cost)), Obscures: text(string(d.Obscures)), //nolint:gosec // bounded by a check
			HazardDice: text(d.HazardDice), HazardType: text(d.HazardType), EveryStep: d.EveryStep, EffectSlug: text(d.Effect),
		}
		if err := q.UpsertSurfaceDefinition(ctx, p); err != nil {
			return err
		}
		if err := q.ClearSurfaceReactions(ctx, string(d.Kind)); err != nil {
			return err
		}
		for _, r := range d.Reactions {
			if err := q.InsertSurfaceReaction(ctx, queries.InsertSurfaceReactionParams{Surface: string(d.Kind), DamageType: r.Damage, Becomes: string(r.Becomes)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("compendium: save surface %s: %w", d.Kind, err)
	}
	return nil
}
