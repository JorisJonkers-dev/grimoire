package app

import (
	"context"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RuleVariantRepository keeps what a Campaign has each Rule Variant at.
type RuleVariantRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	RuleVariants(ctx context.Context, id domain.CampaignID) (variants.Set, error)
	// SetRuleVariants keeps every choice given, all or nothing.
	SetRuleVariants(ctx context.Context, id domain.CampaignID, set variants.Set, now time.Time) error
}

// RuleVariants runs the Rule Variant use cases: every Member sees how the table plays, and the DM
// switches the variants.
type RuleVariants struct {
	Repo RuleVariantRepository
	Now  func() time.Time
}

// RuleVariantView is a built-in Rule Variant with what the Campaign has it at.
type RuleVariantView struct {
	Variant variants.Variant
	Value   string
}

// List shows a Member every built-in Rule Variant and what the Campaign has it at.
func (s *RuleVariants) List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]RuleVariantView, error) {
	if _, err := s.Repo.Membership(ctx, id, c.Subject); err != nil {
		return nil, err
	}
	set, err := s.Repo.RuleVariants(ctx, id)
	if err != nil {
		return nil, err
	}
	catalogue := variants.Catalogue()
	out := make([]RuleVariantView, 0, len(catalogue))
	for _, v := range catalogue {
		out = append(out, RuleVariantView{Variant: v, Value: set.Get(v.Slug)})
	}
	return out, nil
}

// Set switches Rule Variants. Every choice must be one its variant can be, or nothing changes. DM only.
func (s *RuleVariants) Set(ctx context.Context, c caller.Caller, id domain.CampaignID, choices variants.Set) error {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return err
	}
	if me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	for slug, value := range choices {
		if !variants.Valid(slug, value) {
			return domain.ErrInvalid
		}
	}
	return s.Repo.SetRuleVariants(ctx, id, choices, s.Now())
}
