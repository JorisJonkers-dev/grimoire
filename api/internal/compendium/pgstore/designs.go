package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/designs"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// LoadDesigns writes the Effects the Design files state, in one transaction, unless the files are
// unchanged since the last load. The loaded Effects replace any of the same slug, and an Effect an
// earlier load wrote whose file is gone is removed.
func (s *Store) LoadDesigns(ctx context.Context, set designs.Set) (bool, error) {
	last, err := s.q.LatestDesignHash(ctx)
	if err != nil {
		return false, fmt.Errorf("compendium: last design load: %w", err)
	}
	if last == set.Hash {
		return false, nil
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return loadDesigns(ctx, queries.New(tx), set)
	})
	if err != nil {
		return false, fmt.Errorf("compendium: load designs: %w", err)
	}
	return true, nil
}

func loadDesigns(ctx context.Context, q *queries.Queries, set designs.Set) error {
	keep := make([]string, 0, len(set.Effects))
	for _, l := range set.Effects {
		keep = append(keep, l.Definition.Slug)
	}
	if err := q.DeleteDesignEffects(ctx, keep); err != nil {
		return err
	}
	if err := q.ClearDesignEffects(ctx); err != nil {
		return err
	}
	for _, l := range set.Effects {
		if err := saveEffect(ctx, q, l.Owner, l.Definition); err != nil {
			return fmt.Errorf("%s: %w", l.Definition.Slug, err)
		}
		if err := q.MarkDesignEffect(ctx, l.Definition.Slug); err != nil {
			return err
		}
	}
	return q.RecordDesignLoad(ctx, set.Hash)
}
