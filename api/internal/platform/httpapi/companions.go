package httpapi

import (
	"context"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// CompanionService keeps the allies who travel with a party.
type CompanionService interface {
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.Companion, error)
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.CompanionInput) (domain.Companion, error)
	Update(ctx context.Context, c caller.Caller, id domain.CampaignID, companion domain.CompanionID, in campaignapp.CompanionInput) (domain.Companion, error)
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, companion domain.CompanionID) error
}

func companionOut(c domain.Companion) oas.Companion {
	out := oas.Companion{
		ID: oas.ID(c.ID), Name: c.Name, Kind: oas.CompanionKind(c.Kind), MonsterSlug: c.MonsterSlug, SharesXp: c.SharesXP, Notes: c.Notes,
		UpdatedAt: c.UpdatedAt.UTC(),
	}
	if c.Controller != nil {
		out.ControllerId = oas.NewOptID(oas.ID(*c.Controller))
	}
	if c.HP != nil {
		out.Hp = oas.NewOptInt32(int32(*c.HP)) //nolint:gosec // hit points are small
	}
	return out
}

func companionIn(req *oas.CompanionInput) campaignapp.CompanionInput {
	in := campaignapp.CompanionInput{
		Name: req.Name, Kind: string(req.Kind), MonsterSlug: req.MonsterSlug, Controller: nil, SharesXP: req.SharesXp, Notes: req.Notes.Or(""),
	}
	if id, ok := req.ControllerId.Get(); ok {
		m := domain.MemberID(uuid.UUID(id))
		in.Controller = &m
	}
	return in
}

// ListCompanions lists the allies who travel with the party.
func (h *Handler) ListCompanions(ctx context.Context, p oas.ListCompanionsParams) (oas.ListCompanionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Companions.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list companions", err), nil
	}
	out := make([]oas.Companion, 0, len(list))
	for _, x := range list {
		out = append(out, companionOut(x))
	}
	return &oas.ListCompanionsOKHeaders{Response: out}, nil
}

// CreateCompanion adds a Companion.
func (h *Handler) CreateCompanion(ctx context.Context, req *oas.CompanionInput, p oas.CreateCompanionParams) (oas.CreateCompanionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Companions.Create(ctx, c, domain.CampaignID(p.CampaignId), companionIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create companion", err), nil
	}
	return &oas.CompanionHeaders{Response: companionOut(x)}, nil
}

// UpdateCompanion changes a Companion.
func (h *Handler) UpdateCompanion(ctx context.Context, req *oas.CompanionInput, p oas.UpdateCompanionParams) (oas.UpdateCompanionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Companions.Update(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.CompanionId), companionIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "update companion", err), nil
	}
	return &oas.CompanionHeaders{Response: companionOut(x)}, nil
}

// DeleteCompanion lets a Companion go.
func (h *Handler) DeleteCompanion(ctx context.Context, p oas.DeleteCompanionParams) (oas.DeleteCompanionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Companions.Delete(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.CompanionId)); err != nil {
		return h.campaignProblem(ctx, "delete companion", err), nil
	}
	return &oas.DeleteCompanionNoContent{}, nil
}
