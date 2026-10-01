package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

func lootIn(id domain.LootTableID, req *oas.LootTableInput) domain.LootTable {
	t := domain.LootTable{ID: id, Name: req.Name, Rolls: int(req.Rolls), Entries: []domain.LootEntry{}}
	for _, e := range req.Entries {
		entry := domain.LootEntry{Weight: int(e.Weight), Kind: string(e.Kind), Item: string(e.ItemSlug.Or("")), Coin: string(e.Coin.Or("")), Amount: e.Amount.Or("")}
		if x, ok := e.TableId.Get(); ok {
			tid := domain.LootTableID(x)
			entry.Table = &tid
		}
		t.Entries = append(t.Entries, entry)
	}
	return t
}

//nolint:gosec // rolls and weights are bounded by the rules
func lootOut(t domain.LootTable) oas.LootTable {
	out := oas.LootTable{ID: oas.ID(t.ID), Name: t.Name, Rolls: int32(t.Rolls), Entries: []oas.LootEntry{}, UpdatedAt: t.UpdatedAt.UTC()}
	for _, e := range t.Entries {
		entry := oas.LootEntry{Weight: int32(e.Weight), Kind: oas.LootEntryKind(e.Kind)}
		if e.Item != "" {
			entry.ItemSlug = oas.NewOptSlug(oas.Slug(e.Item))
		}
		if e.Coin != "" {
			entry.Coin = oas.NewOptCoin(oas.Coin(e.Coin))
		}
		setOpt(&entry.Amount, e.Amount)
		if e.Table != nil {
			entry.TableId = oas.NewOptID(oas.ID(*e.Table))
		}
		out.Entries = append(out.Entries, entry)
	}
	return out
}

// ListLootTables lists the Campaign's Loot Tables.
func (h *Handler) ListLootTables(ctx context.Context, p oas.ListLootTablesParams) (oas.ListLootTablesRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.LootTables(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list loot tables", err), nil
	}
	out := make([]oas.LootTable, 0, len(list))
	for _, t := range list {
		out = append(out, lootOut(t))
	}
	return &oas.ListLootTablesOKHeaders{Response: out}, nil
}

// CreateLootTable adds a Loot Table.
func (h *Handler) CreateLootTable(ctx context.Context, req *oas.LootTableInput, p oas.CreateLootTableParams) (oas.CreateLootTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	t, err := h.Prep.SaveLootTable(ctx, c, uuid.UUID(p.CampaignId), lootIn(domain.LootTableID{}, req))
	if err != nil {
		return h.campaignProblem(ctx, "create loot table", err), nil
	}
	return &oas.LootTableHeaders{Response: lootOut(t)}, nil
}

// UpdateLootTable replaces a Loot Table.
func (h *Handler) UpdateLootTable(ctx context.Context, req *oas.LootTableInput, p oas.UpdateLootTableParams) (oas.UpdateLootTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	t, err := h.Prep.SaveLootTable(ctx, c, uuid.UUID(p.CampaignId), lootIn(domain.LootTableID(p.LootTableId), req))
	if err != nil {
		return h.campaignProblem(ctx, "update loot table", err), nil
	}
	return &oas.LootTableHeaders{Response: lootOut(t)}, nil
}

// DeleteLootTable removes a Loot Table.
func (h *Handler) DeleteLootTable(ctx context.Context, p oas.DeleteLootTableParams) (oas.DeleteLootTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Prep.DeleteLootTable(ctx, c, uuid.UUID(p.CampaignId), domain.LootTableID(p.LootTableId)); err != nil {
		return h.campaignProblem(ctx, "delete loot table", err), nil
	}
	return &oas.DeleteLootTableNoContent{}, nil
}

// ListLootTableRevisions lists a Loot Table's Revisions.
func (h *Handler) ListLootTableRevisions(ctx context.Context, p oas.ListLootTableRevisionsParams) (oas.ListLootTableRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Prep.LootTableRevisions(ctx, c, uuid.UUID(p.CampaignId), domain.LootTableID(p.LootTableId))
	if err != nil {
		return h.campaignProblem(ctx, "loot table revisions", err), nil
	}
	return &oas.ListLootTableRevisionsOKHeaders{Response: revisionsOut(revs)}, nil
}

// RestoreLootTableRevision brings a Loot Table back to a Revision.
func (h *Handler) RestoreLootTableRevision(ctx context.Context, p oas.RestoreLootTableRevisionParams) (oas.RestoreLootTableRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	t, err := h.Prep.RestoreLootTable(ctx, c, uuid.UUID(p.CampaignId), domain.LootTableID(p.LootTableId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore loot table", err), nil
	}
	return &oas.LootTableHeaders{Response: lootOut(t)}, nil
}
