package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

func draftOut(d domain.Draft) (oas.CharacterDraft, error) {
	out := oas.CharacterDraft{Step: int32(d.Step), Build: oas.CharacterDraftBuild{}, Rolled: nil} //nolint:gosec // 0 to 8
	if len(d.Build) > 0 {
		if err := out.Build.UnmarshalJSON(d.Build); err != nil {
			return out, err
		}
	}
	for _, v := range d.Rolled {
		out.Rolled = append(out.Rolled, int32(v)) //nolint:gosec // 3 to 18
	}
	return out, nil
}

func (h *Handler) draftAnswer(ctx context.Context, op string, d domain.Draft, err error) (*oas.CharacterDraftHeaders, *oas.ProblemStatusCodeWithHeaders) {
	if errors.Is(err, domain.ErrNotFound) {
		return nil, problem(http.StatusNotFound, "No draft", "There is no Character draft here yet.")
	}
	if errors.Is(err, domain.ErrConflict) {
		return nil, problem(http.StatusConflict, "Already rolled", "This draft's scores are rolled; start over to roll again.")
	}
	if err != nil {
		return nil, h.campaignProblem(ctx, op, err)
	}
	out, err := draftOut(d)
	if err != nil {
		return nil, h.campaignProblem(ctx, op, err)
	}
	return &oas.CharacterDraftHeaders{Response: out}, nil
}

// GetCharacterDraft reads the caller's Character draft.
func (h *Handler) GetCharacterDraft(ctx context.Context, p oas.GetCharacterDraftParams) (oas.GetCharacterDraftRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Characters.Draft(ctx, c, domain.CampaignID(p.CampaignId))
	out, bad := h.draftAnswer(ctx, "get draft", d, err)
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// SaveCharacterDraft keeps the wizard's choices.
func (h *Handler) SaveCharacterDraft(ctx context.Context, req *oas.CharacterDraftSave, p oas.SaveCharacterDraftParams) (oas.SaveCharacterDraftRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	build, err := req.Build.MarshalJSON()
	if err != nil {
		return h.campaignProblem(ctx, "save draft", err), nil
	}
	d, err := h.Characters.SaveDraft(ctx, c, domain.CampaignID(p.CampaignId), int(req.Step), build)
	out, bad := h.draftAnswer(ctx, "save draft", d, err)
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// DiscardCharacterDraft starts the wizard over.
func (h *Handler) DiscardCharacterDraft(ctx context.Context, p oas.DiscardCharacterDraftParams) (oas.DiscardCharacterDraftRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Characters.DiscardDraft(ctx, c, domain.CampaignID(p.CampaignId)); err != nil {
		return h.campaignProblem(ctx, "discard draft", err), nil
	}
	return &oas.DiscardCharacterDraftNoContent{}, nil
}

// RollCharacterScores rolls the draft's ability scores.
func (h *Handler) RollCharacterScores(ctx context.Context, p oas.RollCharacterScoresParams) (oas.RollCharacterScoresRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Characters.RollScores(ctx, c, domain.CampaignID(p.CampaignId))
	out, bad := h.draftAnswer(ctx, "roll scores", d, err)
	if bad != nil {
		return bad, nil
	}
	return out, nil
}
