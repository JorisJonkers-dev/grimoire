package pgstore

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

// RuleHooks lists the Rule Variants a DM authored for a Campaign, oldest first.
func (s *Store) RuleHooks(ctx context.Context, id domain.CampaignID) ([]domain.RuleHook, error) {
	rows, err := s.q.ListRuleHooks(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.RuleHook, 0, len(rows))
	for _, r := range rows {
		h := domain.RuleHook{ID: r.ID, CampaignID: domain.CampaignID(r.CampaignID), Name: r.Name, Hook: r.Hook, RollTable: nil, Effect: r.Effect.String, CreatedAt: r.CreatedAt}
		if r.RollTable.Valid {
			table := uuid.UUID(r.RollTable.Bytes)
			h.RollTable = &table
		}
		out = append(out, h)
	}
	return out, nil
}

// CampaignRollTables names the Roll Tables of the Library a Campaign sees, by name.
func (s *Store) CampaignRollTables(ctx context.Context, id domain.CampaignID) ([]domain.RollTableRef, error) {
	rows, err := s.q.CampaignHomebrewDesigns(ctx, queries.CampaignHomebrewDesignsParams{CampaignID: uuid.UUID(id), Kind: "table"})
	if err != nil {
		return nil, err
	}
	out := make([]domain.RollTableRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.RollTableRef{ID: r.ID, Name: r.Name})
	}
	slices.SortStableFunc(out, func(a, b domain.RollTableRef) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// InsertRuleHook adds a Rule Variant a DM authored.
func (s *Store) InsertRuleHook(ctx context.Context, h domain.RuleHook) error {
	p := queries.InsertRuleHookParams{ID: h.ID, CampaignID: uuid.UUID(h.CampaignID), Name: h.Name, Hook: h.Hook, Now: h.CreatedAt}
	if h.RollTable != nil {
		p.RollTable = pgtype.UUID{Bytes: *h.RollTable, Valid: true}
	}
	if h.Effect != "" {
		p.Effect = pgtype.Text{String: h.Effect, Valid: true}
	}
	return s.q.InsertRuleHook(ctx, p)
}

// DeleteRuleHook removes a Rule Variant a DM authored; ErrNotFound when the Campaign has no such one.
func (s *Store) DeleteRuleHook(ctx context.Context, id domain.CampaignID, hook domain.HookID) error {
	n, err := s.q.DeleteRuleHook(ctx, queries.DeleteRuleHookParams{CampaignID: uuid.UUID(id), ID: hook})
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}
