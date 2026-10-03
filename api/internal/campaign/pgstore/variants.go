package pgstore

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
)

// RuleVariants reads what a Campaign has each Rule Variant at.
func (s *Store) RuleVariants(ctx context.Context, id domain.CampaignID) (variants.Set, error) {
	rows, err := s.q.ListRuleVariants(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make(variants.Set, len(rows))
	for _, r := range rows {
		out[r.Variant] = r.Value
	}
	return out, nil
}

// SetRuleVariants keeps every choice given, all or nothing.
func (s *Store) SetRuleVariants(ctx context.Context, id domain.CampaignID, set variants.Set, now time.Time) error {
	slugs := make([]string, 0, len(set))
	for slug := range set {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := queries.New(s.wrap(tx))
		for _, slug := range slugs {
			if err := q.SetRuleVariant(ctx, queries.SetRuleVariantParams{CampaignID: uuid.UUID(id), Variant: slug, Value: set[slug], Now: now}); err != nil {
				return err
			}
		}
		return nil
	})
}
