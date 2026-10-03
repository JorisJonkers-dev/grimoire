package httpapi

import (
	"context"

	"github.com/google/uuid"

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

// RuleHookService keeps the Rule Variants a DM authors from hook points.
type RuleHookService interface {
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) (campaignapp.RuleHooksView, error)
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.RuleHookInput) (domain.RuleHook, error)
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, hook domain.HookID) error
}

func ruleHookOut(h domain.RuleHook, tableName string) oas.RuleHook {
	out := oas.RuleHook{ID: oas.ID(h.ID), Name: h.Name, Hook: oas.HookPointSlug(h.Hook)}
	if h.RollTable != nil {
		out.RollTableId, out.TableName = oas.NewOptID(oas.ID(*h.RollTable)), oas.NewOptString(tableName)
	}
	if h.Effect != "" {
		out.Effect = oas.NewOptString(h.Effect)
	}
	return out
}

// ListRuleHooks lists the Campaign's own Rule Variants.
func (h *Handler) ListRuleHooks(ctx context.Context, p oas.ListRuleHooksParams) (oas.ListRuleHooksRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.RuleHooks.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list rule hooks", err), nil
	}
	out := oas.RuleHooks{
		Dm: v.DM, Hooks: make([]oas.RuleHook, 0, len(v.Hooks)), Points: make([]oas.RuleHooksPointsItem, 0, len(v.Points)),
		Tables: make([]oas.RuleHooksTablesItem, 0, len(v.Tables)),
	}
	for _, row := range v.Hooks {
		out.Hooks = append(out.Hooks, ruleHookOut(row.Hook, row.TableName))
	}
	for _, point := range v.Points {
		out.Points = append(out.Points, oas.RuleHooksPointsItem{Slug: oas.HookPointSlug(point.Slug), Label: point.Label})
	}
	for _, table := range v.Tables {
		out.Tables = append(out.Tables, oas.RuleHooksTablesItem{ID: oas.ID(table.ID), Name: table.Name})
	}
	return &oas.RuleHooksHeaders{Response: out}, nil
}

// CreateRuleHook authors a Rule Variant of the Campaign's own.
func (h *Handler) CreateRuleHook(ctx context.Context, req *oas.RuleHookInput, p oas.CreateRuleHookParams) (oas.CreateRuleHookRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := campaignapp.RuleHookInput{Name: req.Name, Hook: string(req.Hook), RollTable: nil, Effect: req.Effect.Or("")}
	if id, set := req.RollTableId.Get(); set {
		table := uuid.UUID(id)
		in.RollTable = &table
	}
	made, err := h.RuleHooks.Create(ctx, c, domain.CampaignID(p.CampaignId), in)
	if err != nil {
		return h.campaignProblem(ctx, "create rule hook", err), nil
	}
	return &oas.RuleHookHeaders{Response: ruleHookOut(made, "")}, nil
}

// DeleteRuleHook removes one of the Campaign's own Rule Variants.
func (h *Handler) DeleteRuleHook(ctx context.Context, p oas.DeleteRuleHookParams) (oas.DeleteRuleHookRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.RuleHooks.Delete(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.HookId)); err != nil {
		return h.campaignProblem(ctx, "delete rule hook", err), nil
	}
	return &oas.DeleteRuleHookNoContent{}, nil
}
