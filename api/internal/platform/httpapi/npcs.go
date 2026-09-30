package httpapi

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// NPCService is what the NPC and revision operations need.
type NPCService interface {
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in app.NPCInput) (domain.NPC, error)
	Update(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, in app.NPCInput) (domain.NPC, error)
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) error
	Restore(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, no int) (domain.NPC, error)
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.NPC, error)
	Deleted(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.DeletedNPC, error)
	Get(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) (domain.NPC, error)
	Revisions(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID) ([]domain.Revision, error)
	Diff(ctx context.Context, c caller.Caller, id domain.CampaignID, npcID domain.NPCID, from, to int) ([]domain.Change, error)
}

func npcIn(req *oas.NpcInput) app.NPCInput {
	return app.NPCInput{
		Name: req.Name, Title: req.Title.Or(""), Description: req.Description.Or(""), DMNotes: req.DmNotes.Or(""),
		Disposition: string(req.Disposition),
	}
}

func npcOut(n domain.NPC) oas.Npc {
	return oas.Npc{
		ID: oas.ID(n.ID), Name: n.Name, Title: n.Title, Description: n.Description, DmNotes: n.DMNotes,
		Disposition: oas.Disposition(n.Disposition), UpdatedAt: n.UpdatedAt.UTC(),
	}
}

// ListNpcs lists the Campaign's NPCs.
func (h *Handler) ListNpcs(ctx context.Context, p oas.ListNpcsParams) (oas.ListNpcsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.NPCs.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list npcs", err), nil
	}
	out := make([]oas.Npc, 0, len(list))
	for _, n := range list {
		out = append(out, npcOut(n))
	}
	return &oas.ListNpcsOKHeaders{Response: out}, nil
}

// ListDeletedNpcs lists NPCs that can be restored.
func (h *Handler) ListDeletedNpcs(ctx context.Context, p oas.ListDeletedNpcsParams) (oas.ListDeletedNpcsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.NPCs.Deleted(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list deleted npcs", err), nil
	}
	out := make([]oas.DeletedNpc, 0, len(list))
	for _, n := range list {
		out = append(out, oas.DeletedNpc{ID: oas.ID(n.ID), Name: n.Name, DeletedAt: n.DeletedAt.UTC()})
	}
	return &oas.ListDeletedNpcsOKHeaders{Response: out}, nil
}

// CreateNpc adds an NPC.
func (h *Handler) CreateNpc(ctx context.Context, req *oas.NpcInput, p oas.CreateNpcParams) (oas.CreateNpcRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.NPCs.Create(ctx, c, domain.CampaignID(p.CampaignId), npcIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create npc", err), nil
	}
	return &oas.NpcHeaders{Response: npcOut(n)}, nil
}

// GetNpc returns one NPC.
func (h *Handler) GetNpc(ctx context.Context, p oas.GetNpcParams) (oas.GetNpcRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.NPCs.Get(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId))
	if err != nil {
		return h.campaignProblem(ctx, "get npc", err), nil
	}
	return &oas.NpcHeaders{Response: npcOut(n)}, nil
}

// UpdateNpc replaces an NPC.
func (h *Handler) UpdateNpc(ctx context.Context, req *oas.NpcInput, p oas.UpdateNpcParams) (oas.UpdateNpcRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.NPCs.Update(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId), npcIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "update npc", err), nil
	}
	return &oas.NpcHeaders{Response: npcOut(n)}, nil
}

// DeleteNpc removes an NPC.
func (h *Handler) DeleteNpc(ctx context.Context, p oas.DeleteNpcParams) (oas.DeleteNpcRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.NPCs.Delete(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId)); err != nil {
		return h.campaignProblem(ctx, "delete npc", err), nil
	}
	return &oas.DeleteNpcNoContent{}, nil
}

// ListNpcRevisions lists an NPC's Revisions.
func (h *Handler) ListNpcRevisions(ctx context.Context, p oas.ListNpcRevisionsParams) (oas.ListNpcRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.NPCs.Revisions(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId))
	if err != nil {
		return h.campaignProblem(ctx, "list revisions", err), nil
	}
	out := make([]oas.Revision, 0, len(revs))
	for _, r := range revs {
		rev := oas.Revision{
			No: int32(r.No), Action: oas.RevisionAction(r.Action), Author: oas.DisplayName(r.Author), //nolint:gosec // bounded
			Origin: oas.RevisionOrigin(r.Origin), CreatedAt: r.CreatedAt.UTC(),
		}
		setOpt(&rev.Client, r.Client)
		if r.RestoredFrom > 0 {
			rev.RestoredFrom = oas.NewOptInt32(int32(r.RestoredFrom)) //nolint:gosec // bounded
		}
		out = append(out, rev)
	}
	return &oas.ListNpcRevisionsOKHeaders{Response: out}, nil
}

// DiffNpcRevisions compares two Revisions.
func (h *Handler) DiffNpcRevisions(ctx context.Context, p oas.DiffNpcRevisionsParams) (oas.DiffNpcRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	changes, err := h.NPCs.Diff(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId), int(p.From), int(p.To))
	if err != nil {
		return h.campaignProblem(ctx, "diff revisions", err), nil
	}
	out := make([]oas.FieldChange, 0, len(changes))
	for _, ch := range changes {
		out = append(out, oas.FieldChange{Field: ch.Field, Before: ch.Before, After: ch.After})
	}
	return &oas.DiffNpcRevisionsOKHeaders{Response: out}, nil
}

// RestoreNpcRevision restores an NPC to a Revision.
func (h *Handler) RestoreNpcRevision(ctx context.Context, p oas.RestoreNpcRevisionParams) (oas.RestoreNpcRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.NPCs.Restore(ctx, c, domain.CampaignID(p.CampaignId), domain.NPCID(p.NpcId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore revision", err), nil
	}
	return &oas.NpcHeaders{Response: npcOut(n)}, nil
}
