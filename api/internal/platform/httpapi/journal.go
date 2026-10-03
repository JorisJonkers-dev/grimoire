package httpapi

import (
	"context"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// JournalService keeps a Campaign's Quests and Lore.
type JournalService interface {
	Read(ctx context.Context, c caller.Caller, id domain.CampaignID) (campaignapp.JournalView, error)
	CreateQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.QuestInput) (domain.Quest, error)
	UpdateQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, quest domain.QuestID, in campaignapp.QuestInput) (domain.Quest, error)
	DeleteQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, quest domain.QuestID) error
	CreateLore(ctx context.Context, c caller.Caller, id domain.CampaignID, in campaignapp.LoreInput) (domain.Lore, error)
	UpdateLore(ctx context.Context, c caller.Caller, id domain.CampaignID, lore domain.LoreID, in campaignapp.LoreInput) (domain.Lore, error)
	DeleteLore(ctx context.Context, c caller.Caller, id domain.CampaignID, lore domain.LoreID) error
	ReadItem(ctx context.Context, c caller.Caller, id domain.CampaignID, itemSlug string) (int, error)
}

func questOut(q domain.Quest) oas.Quest {
	out := oas.Quest{ID: oas.ID(q.ID), Name: q.Name, Summary: q.Summary, Status: oas.QuestStatus(q.Status), Steps: make([]oas.QuestStep, 0, len(q.Steps)), UpdatedAt: q.UpdatedAt.UTC()}
	for _, step := range q.Steps {
		out.Steps = append(out.Steps, oas.QuestStep{Text: step.Text, Done: step.Done})
	}
	return out
}

// loreOut shapes a Lore entry; the item that holds it goes to the DM alone.
func loreOut(l domain.Lore, dm bool) oas.Lore {
	out := oas.Lore{ID: oas.ID(l.ID), Title: l.Title, Body: l.Body, Unlocked: l.UnlockedAt != nil, UpdatedAt: l.UpdatedAt.UTC()}
	if l.UnlockedAt != nil {
		out.UnlockedAt = oas.NewOptDateTime(l.UnlockedAt.UTC())
	}
	if dm {
		out.ItemSlug = oas.NewOptString(l.ItemSlug)
	}
	return out
}

func questIn(req *oas.QuestInput) campaignapp.QuestInput {
	in := campaignapp.QuestInput{Name: req.Name, Summary: req.Summary.Or(""), Status: string(req.Status), Steps: make([]domain.QuestStep, 0, len(req.Steps))}
	for _, step := range req.Steps {
		in.Steps = append(in.Steps, domain.QuestStep{Text: step.Text, Done: step.Done})
	}
	return in
}

func loreIn(req *oas.LoreInput) campaignapp.LoreInput {
	return campaignapp.LoreInput{Title: req.Title, Body: req.Body.Or(""), ItemSlug: req.ItemSlug.Or(""), Unlocked: req.Unlocked.Or(false)}
}

// GetJournal shows the Journal as the caller may see it.
func (h *Handler) GetJournal(ctx context.Context, p oas.GetJournalParams) (oas.GetJournalRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	v, err := h.Journal.Read(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "read journal", err), nil
	}
	out := oas.Journal{Dm: v.DM, Quests: make([]oas.Quest, 0, len(v.Quests)), Lore: make([]oas.Lore, 0, len(v.Lore)), Readable: v.Readable}
	for _, q := range v.Quests {
		out.Quests = append(out.Quests, questOut(q))
	}
	for _, l := range v.Lore {
		out.Lore = append(out.Lore, loreOut(l, v.DM))
	}
	return &oas.JournalHeaders{Response: out}, nil
}

// ReadItem is a Member reading a book or letter they carry.
func (h *Handler) ReadItem(ctx context.Context, req *oas.ItemReading, p oas.ReadItemParams) (oas.ReadItemRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.Journal.ReadItem(ctx, c, domain.CampaignID(p.CampaignId), req.ItemSlug)
	if err != nil {
		return h.campaignProblem(ctx, "read item", err), nil
	}
	return &oas.ItemReadingResultHeaders{Response: oas.ItemReadingResult{Unlocked: int32(n)}}, nil //nolint:gosec // at most the Campaign's Lore entries
}

// CreateQuest adds a Quest.
func (h *Handler) CreateQuest(ctx context.Context, req *oas.QuestInput, p oas.CreateQuestParams) (oas.CreateQuestRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	q, err := h.Journal.CreateQuest(ctx, c, domain.CampaignID(p.CampaignId), questIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create quest", err), nil
	}
	return &oas.QuestHeaders{Response: questOut(q)}, nil
}

// UpdateQuest changes a Quest.
func (h *Handler) UpdateQuest(ctx context.Context, req *oas.QuestInput, p oas.UpdateQuestParams) (oas.UpdateQuestRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if _, err := h.Journal.UpdateQuest(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.QuestId), questIn(req)); err != nil {
		return h.campaignProblem(ctx, "update quest", err), nil
	}
	return &oas.UpdateQuestNoContent{}, nil
}

// DeleteQuest removes a Quest.
func (h *Handler) DeleteQuest(ctx context.Context, p oas.DeleteQuestParams) (oas.DeleteQuestRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Journal.DeleteQuest(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.QuestId)); err != nil {
		return h.campaignProblem(ctx, "delete quest", err), nil
	}
	return &oas.DeleteQuestNoContent{}, nil
}

// CreateLore adds a Lore entry.
func (h *Handler) CreateLore(ctx context.Context, req *oas.LoreInput, p oas.CreateLoreParams) (oas.CreateLoreRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	l, err := h.Journal.CreateLore(ctx, c, domain.CampaignID(p.CampaignId), loreIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create lore", err), nil
	}
	return &oas.LoreHeaders{Response: loreOut(l, true)}, nil
}

// UpdateLore changes a Lore entry.
func (h *Handler) UpdateLore(ctx context.Context, req *oas.LoreInput, p oas.UpdateLoreParams) (oas.UpdateLoreRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if _, err := h.Journal.UpdateLore(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.LoreId), loreIn(req)); err != nil {
		return h.campaignProblem(ctx, "update lore", err), nil
	}
	return &oas.UpdateLoreNoContent{}, nil
}

// DeleteLore removes a Lore entry.
func (h *Handler) DeleteLore(ctx context.Context, p oas.DeleteLoreParams) (oas.DeleteLoreRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Journal.DeleteLore(ctx, c, domain.CampaignID(p.CampaignId), uuid.UUID(p.LoreId)); err != nil {
		return h.campaignProblem(ctx, "delete lore", err), nil
	}
	return &oas.DeleteLoreNoContent{}, nil
}
