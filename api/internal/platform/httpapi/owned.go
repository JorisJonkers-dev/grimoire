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

func barsOut(b domain.ActionBars) *oas.ActionBarsHeaders {
	out := oas.ActionBars{Bars: make([][]oas.ActionBarTile, 0, len(b.Bars)), Quick: tilesOut(b.Quick), Stowed: tilesOut(b.Stowed), Arranged: b.Arranged}
	for _, bar := range b.Bars {
		out.Bars = append(out.Bars, tilesOut(bar))
	}
	return &oas.ActionBarsHeaders{Response: out}
}

func tilesOut(tiles []string) []oas.ActionBarTile {
	out := make([]oas.ActionBarTile, 0, len(tiles))
	for _, t := range tiles {
		out = append(out, oas.ActionBarTile(t))
	}
	return out
}

func tilesIn(tiles []oas.ActionBarTile) []string {
	out := make([]string, 0, len(tiles))
	for _, t := range tiles {
		out = append(out, string(t))
	}
	return out
}

// GetActionBars reads how the caller laid out one of their Characters' action bars.
func (h *Handler) GetActionBars(ctx context.Context, p oas.GetActionBarsParams) (oas.GetActionBarsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	b, err := h.Characters.ActionBars(ctx, c, domain.OwnedID(p.CharacterId))
	if err != nil {
		return h.ownedProblem(ctx, "get action bars", err), nil
	}
	return barsOut(b), nil
}

// SetActionBars saves the caller's layout for one of their Characters.
func (h *Handler) SetActionBars(ctx context.Context, req *oas.ActionBarsChange, p oas.SetActionBarsParams) (oas.SetActionBarsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := domain.ActionBars{Bars: make([][]string, 0, len(req.Bars)), Quick: tilesIn(req.Quick), Stowed: tilesIn(req.Stowed), Arranged: true}
	for _, bar := range req.Bars {
		in.Bars = append(in.Bars, tilesIn(bar))
	}
	b, err := h.Characters.SetActionBars(ctx, c, domain.OwnedID(p.CharacterId), in)
	if err != nil {
		return h.ownedProblem(ctx, "set action bars", err), nil
	}
	return barsOut(b), nil
}
