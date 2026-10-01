package httpapi

import (
	"context"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

//nolint:gosec // revision numbers are bounded
func activityOut(e campaigndomain.Edit) oas.Activity {
	out := oas.Activity{
		RevisionId: oas.ID(e.RevisionID), EntityType: oas.ActivityEntityType(e.EntityType), EntityId: oas.ID(e.EntityID), Name: e.Name,
		No: int32(e.No), Action: oas.ActivityAction(e.Action), Author: oas.DisplayName(e.Author), Origin: oas.ActivityOrigin(e.Origin),
		CreatedAt: e.CreatedAt.UTC(), Undoable: e.Latest,
	}
	if e.Client != "" {
		out.Client = oas.NewOptString(e.Client)
	}
	return out
}

// ListActivity lists the latest prep changes made through MCP.
func (h *Handler) ListActivity(ctx context.Context, p oas.ListActivityParams) (oas.ListActivityRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Campaigns.Activity(ctx, c, campaignID(uuid.UUID(p.CampaignId)))
	if err != nil {
		return h.campaignProblem(ctx, "list activity", err), nil
	}
	out := make([]oas.Activity, 0, len(list))
	for _, e := range list {
		out = append(out, activityOut(e))
	}
	return &oas.ListActivityOKHeaders{Response: out}, nil
}

// UndoChange deletes what a change created, or restores the Revision before it.
func (h *Handler) UndoChange(ctx context.Context, p oas.UndoChangeParams) (oas.UndoChangeRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	cid := uuid.UUID(p.CampaignId)
	e, no, err := h.Campaigns.UndoPlan(ctx, c, campaignID(cid), uuid.UUID(p.RevisionId))
	if err == nil {
		err = h.undo(ctx, c, cid, e, no)
	}
	if err == nil {
		e, err = h.Campaigns.LatestEdit(ctx, c, campaignID(cid), e.EntityType, e.EntityID)
	}
	if err != nil {
		return h.campaignProblem(ctx, "undo change", err), nil
	}
	return &oas.ActivityHeaders{Response: activityOut(e)}, nil
}

// errNoUndo refuses changes to anything but prep data.
var errNoUndo = apperr.Refuse("This change cannot be undone.")

// undo deletes the entity when no is 0, or else restores its Revision no.
func (h *Handler) undo(ctx context.Context, c caller.Caller, campaign uuid.UUID, e campaigndomain.Edit, no int) error {
	id := e.EntityID
	pick := func(del func() error, restore func() error) error {
		if no == 0 {
			return del()
		}
		return restore()
	}
	switch e.EntityType {
	case campaigndomain.EntityNPC:
		return pick(func() error { return h.NPCs.Delete(ctx, c, campaignID(campaign), campaigndomain.NPCID(id)) },
			func() error {
				_, err := h.NPCs.Restore(ctx, c, campaignID(campaign), campaigndomain.NPCID(id), no)
				return err
			})
	case prep.EntityPool:
		return pick(func() error { return h.Prep.DeletePool(ctx, c, campaign, prep.PoolID(id)) },
			func() error { _, err := h.Prep.RestorePool(ctx, c, campaign, prep.PoolID(id), no); return err })
	case prep.EntityTable:
		return pick(func() error { return h.Prep.DeleteTable(ctx, c, campaign, prep.TableID(id)) },
			func() error { _, err := h.Prep.RestoreTable(ctx, c, campaign, prep.TableID(id), no); return err })
	case prep.EntityLoot:
		return pick(func() error { return h.Prep.DeleteLootTable(ctx, c, campaign, prep.LootTableID(id)) },
			func() error {
				_, err := h.Prep.RestoreLootTable(ctx, c, campaign, prep.LootTableID(id), no)
				return err
			})
	case prep.EntitySettlement:
		return pick(func() error { return h.Prep.DeleteSettlement(ctx, c, campaign, prep.SettlementID(id)) },
			func() error {
				_, err := h.Prep.RestoreSettlement(ctx, c, campaign, prep.SettlementID(id), no)
				return err
			})
	case prep.EntityShop:
		return pick(func() error { return h.Prep.DeleteShop(ctx, c, campaign, prep.ShopID(id)) },
			func() error { _, err := h.Prep.RestoreShop(ctx, c, campaign, prep.ShopID(id), no); return err })
	}
	return errNoUndo
}
