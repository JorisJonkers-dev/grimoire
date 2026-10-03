package httpapi

import (
	"context"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RuleVariantService keeps what a Campaign has each Rule Variant at.
type RuleVariantService interface {
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]campaignapp.RuleVariantView, error)
	Set(ctx context.Context, c caller.Caller, id domain.CampaignID, choices variants.Set) error
}

// ListRuleVariants lists the built-in Rule Variants with what the Campaign has each at.
func (h *Handler) ListRuleVariants(ctx context.Context, p oas.ListRuleVariantsParams) (oas.ListRuleVariantsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.RuleVariants.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list rule variants", err), nil
	}
	out := make([]oas.RuleVariant, 0, len(list))
	for _, v := range list {
		row := oas.RuleVariant{
			Slug: v.Variant.Slug, Name: v.Variant.Name, Description: v.Variant.Description, Automated: v.Variant.Automated, Value: v.Value,
			Options: make([]oas.RuleVariantOption, 0, len(v.Variant.Options)),
		}
		for _, o := range v.Variant.Options {
			row.Options = append(row.Options, oas.RuleVariantOption{Value: o.Value, Label: o.Label})
		}
		out = append(out, row)
	}
	return &oas.ListRuleVariantsOKHeaders{Response: out}, nil
}

// SetRuleVariants switches Rule Variants.
func (h *Handler) SetRuleVariants(ctx context.Context, req *oas.RuleVariantChoices, p oas.SetRuleVariantsParams) (oas.SetRuleVariantsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	choices := make(variants.Set, len(req.Choices))
	for _, choice := range req.Choices {
		choices[choice.Slug] = choice.Value
	}
	if err := h.RuleVariants.Set(ctx, c, domain.CampaignID(p.CampaignId), choices); err != nil {
		return h.campaignProblem(ctx, "set rule variants", err), nil
	}
	return &oas.SetRuleVariantsNoContent{}, nil
}
