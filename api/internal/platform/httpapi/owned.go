package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

func ownedOut(o domain.OwnedCharacter) oas.OwnedCharacter {
	out := oas.OwnedCharacter{
		ID: oas.ID(o.ID), Name: o.Name, Ruleset: o.Ruleset, Species: o.Species, Class: o.Class, Background: o.Background,
		Backstory: o.Backstory, HasPortrait: o.HasPortrait, Campaigns: make([]oas.CampaignEntry, 0, len(o.Campaigns)),
	}
	for _, e := range o.Campaigns {
		out.Campaigns = append(out.Campaigns, oas.CampaignEntry{
			CampaignId: oas.ID(e.CampaignID), CampaignName: e.CampaignName, CharacterId: oas.ID(e.CharacterID),
			Level: int32(e.Level), HpCurrent: int32(e.HPCurrent), HpMax: int32(e.HPMax), //nolint:gosec // small by the rules
		})
	}
	return out
}

func (h *Handler) ownedProblem(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "You have no such Character, or no such Campaign.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Already there", "This Character already plays in that Campaign.")
	}
	return h.campaignProblem(ctx, op, err)
}

// ListMyCharacters lists the caller's Characters.
func (h *Handler) ListMyCharacters(ctx context.Context) (oas.ListMyCharactersRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	mine, err := h.Characters.Mine(ctx, c)
	if err != nil {
		return h.ownedProblem(ctx, "list my characters", err), nil
	}
	out := oas.OwnedCharacterList{Items: make([]oas.OwnedCharacter, 0, len(mine))}
	for _, o := range mine {
		out.Items = append(out.Items, ownedOut(o))
	}
	return &oas.OwnedCharacterListHeaders{Response: out}, nil
}

// GetMyCharacter reads one of the caller's Characters.
func (h *Handler) GetMyCharacter(ctx context.Context, p oas.GetMyCharacterParams) (oas.GetMyCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	o, err := h.Characters.Owned(ctx, c, domain.OwnedID(p.CharacterId))
	if err != nil {
		return h.ownedProblem(ctx, "get my character", err), nil
	}
	return &oas.OwnedCharacterHeaders{Response: ownedOut(o)}, nil
}

// UpdateMyCharacter changes a Character's name and Backstory.
func (h *Handler) UpdateMyCharacter(ctx context.Context, req *oas.OwnedCharacterChange, p oas.UpdateMyCharacterParams) (oas.UpdateMyCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	o, err := h.Characters.UpdateOwned(ctx, c, domain.OwnedID(p.CharacterId), req.Name, req.Backstory)
	if err != nil {
		return h.ownedProblem(ctx, "update my character", err), nil
	}
	return &oas.OwnedCharacterHeaders{Response: ownedOut(o)}, nil
}

// JoinCampaign brings a Character into another Campaign.
func (h *Handler) JoinCampaign(ctx context.Context, req *oas.CharacterJoin, p oas.JoinCampaignParams) (oas.JoinCampaignRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Characters.Join(ctx, c, domain.OwnedID(p.CharacterId), domain.CampaignID(uuid.UUID(req.CampaignId)))
	if err != nil {
		return h.ownedProblem(ctx, "join campaign", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}
