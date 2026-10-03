package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

func settlementIn(id domain.SettlementID, req *oas.SettlementInput) domain.Settlement {
	x := domain.Settlement{ID: id, Name: req.Name, Size: string(req.Size), Wealth: string(req.Wealth)}
	if l, ok := req.LocationId.Get(); ok {
		u := uuid.UUID(l)
		x.LocationID = &u
	}
	return x
}

func settlementOut(x domain.Settlement) oas.Settlement {
	out := oas.Settlement{ID: oas.ID(x.ID), Name: x.Name, Size: oas.SettlementSize(x.Size), Wealth: oas.SettlementWealth(x.Wealth), UpdatedAt: x.UpdatedAt.UTC()}
	if x.LocationID != nil {
		out.LocationId = oas.NewOptID(oas.ID(*x.LocationID))
	}
	return out
}

func shopIn(id domain.ShopID, req *oas.ShopInput) domain.Shop {
	x := domain.Shop{
		ID: id, SettlementID: domain.SettlementID(req.SettlementId), Name: req.Name, Kind: req.Kind, MarkupPct: int(req.MarkupPct),
		HaggleDC: int(req.HaggleDc), HagglePct: int(req.HagglePct), Restock: string(req.Restock), RestockDays: int(req.RestockDays.Or(0)),
	}
	if o, ok := req.OwnerId.Get(); ok {
		u := uuid.UUID(o)
		x.OwnerID = &u
	}
	if f, ok := req.FactionId.Get(); ok {
		u := uuid.UUID(f)
		x.FactionID = &u
	}
	if l, ok := req.LootTableId.Get(); ok {
		t := domain.LootTableID(l)
		x.LootTable = &t
	}
	return x
}

//nolint:gosec // percentages, days, counts and prices are bounded by the rules
func shopOut(x domain.Shop) oas.Shop {
	out := oas.Shop{
		ID: oas.ID(x.ID), SettlementId: oas.ID(x.SettlementID), Name: x.Name, Kind: x.Kind, MarkupPct: int32(x.MarkupPct), HaggleDc: int32(x.HaggleDC),
		HagglePct: int32(x.HagglePct), Restock: oas.ShopRestock(x.Restock), StockedDay: int32(x.StockedDay), Stock: []oas.StockItem{}, UpdatedAt: x.UpdatedAt.UTC(),
	}
	if x.OwnerID != nil {
		out.OwnerId = oas.NewOptID(oas.ID(*x.OwnerID))
	}
	if x.FactionID != nil {
		out.FactionId = oas.NewOptID(oas.ID(*x.FactionID))
	}
	if x.LootTable != nil {
		out.LootTableId = oas.NewOptID(oas.ID(*x.LootTable))
	}
	if x.RestockDays > 0 {
		out.RestockDays = oas.NewOptInt32(int32(x.RestockDays))
	}
	for _, k := range x.Stock {
		out.Stock = append(out.Stock, oas.StockItem{ItemSlug: oas.Slug(k.Slug), Quantity: int32(k.Quantity), PriceCp: int32(k.PriceCP)})
	}
	return out
}

// ListSettlements lists the Campaign's Settlements.
func (h *Handler) ListSettlements(ctx context.Context, p oas.ListSettlementsParams) (oas.ListSettlementsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Settlements(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list settlements", err), nil
	}
	out := make([]oas.Settlement, 0, len(list))
	for _, x := range list {
		out = append(out, settlementOut(x))
	}
	return &oas.ListSettlementsOKHeaders{Response: out}, nil
}

// CreateSettlement adds a Settlement.
func (h *Handler) CreateSettlement(ctx context.Context, req *oas.SettlementInput, p oas.CreateSettlementParams) (oas.CreateSettlementRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.SaveSettlement(ctx, c, uuid.UUID(p.CampaignId), settlementIn(domain.SettlementID{}, req))
	if err != nil {
		return h.campaignProblem(ctx, "create settlement", err), nil
	}
	return &oas.SettlementHeaders{Response: settlementOut(x)}, nil
}

// UpdateSettlement replaces a Settlement.
func (h *Handler) UpdateSettlement(ctx context.Context, req *oas.SettlementInput, p oas.UpdateSettlementParams) (oas.UpdateSettlementRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.SaveSettlement(ctx, c, uuid.UUID(p.CampaignId), settlementIn(domain.SettlementID(p.SettlementId), req))
	if err != nil {
		return h.campaignProblem(ctx, "update settlement", err), nil
	}
	return &oas.SettlementHeaders{Response: settlementOut(x)}, nil
}

// DeleteSettlement removes a Settlement.
func (h *Handler) DeleteSettlement(ctx context.Context, p oas.DeleteSettlementParams) (oas.DeleteSettlementRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Prep.DeleteSettlement(ctx, c, uuid.UUID(p.CampaignId), domain.SettlementID(p.SettlementId)); err != nil {
		return h.campaignProblem(ctx, "delete settlement", err), nil
	}
	return &oas.DeleteSettlementNoContent{}, nil
}

// ListSettlementRevisions lists a Settlement's Revisions.
func (h *Handler) ListSettlementRevisions(ctx context.Context, p oas.ListSettlementRevisionsParams) (oas.ListSettlementRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Prep.SettlementRevisions(ctx, c, uuid.UUID(p.CampaignId), domain.SettlementID(p.SettlementId))
	if err != nil {
		return h.campaignProblem(ctx, "settlement revisions", err), nil
	}
	return &oas.ListSettlementRevisionsOKHeaders{Response: revisionsOut(revs)}, nil
}

// RestoreSettlementRevision brings a Settlement back to a Revision.
func (h *Handler) RestoreSettlementRevision(ctx context.Context, p oas.RestoreSettlementRevisionParams) (oas.RestoreSettlementRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.RestoreSettlement(ctx, c, uuid.UUID(p.CampaignId), domain.SettlementID(p.SettlementId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore settlement", err), nil
	}
	return &oas.SettlementHeaders{Response: settlementOut(x)}, nil
}

// ListShops lists the Campaign's Shops.
func (h *Handler) ListShops(ctx context.Context, p oas.ListShopsParams) (oas.ListShopsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Shops(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list shops", err), nil
	}
	out := make([]oas.Shop, 0, len(list))
	for _, x := range list {
		out = append(out, shopOut(x))
	}
	return &oas.ListShopsOKHeaders{Response: out}, nil
}

// CreateShop adds a Shop.
func (h *Handler) CreateShop(ctx context.Context, req *oas.ShopInput, p oas.CreateShopParams) (oas.CreateShopRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.SaveShop(ctx, c, uuid.UUID(p.CampaignId), shopIn(domain.ShopID{}, req))
	if err != nil {
		return h.campaignProblem(ctx, "create shop", err), nil
	}
	return &oas.ShopHeaders{Response: shopOut(x)}, nil
}

// UpdateShop replaces a Shop.
func (h *Handler) UpdateShop(ctx context.Context, req *oas.ShopInput, p oas.UpdateShopParams) (oas.UpdateShopRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.SaveShop(ctx, c, uuid.UUID(p.CampaignId), shopIn(domain.ShopID(p.ShopId), req))
	if err != nil {
		return h.campaignProblem(ctx, "update shop", err), nil
	}
	return &oas.ShopHeaders{Response: shopOut(x)}, nil
}

// DeleteShop removes a Shop.
func (h *Handler) DeleteShop(ctx context.Context, p oas.DeleteShopParams) (oas.DeleteShopRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Prep.DeleteShop(ctx, c, uuid.UUID(p.CampaignId), domain.ShopID(p.ShopId)); err != nil {
		return h.campaignProblem(ctx, "delete shop", err), nil
	}
	return &oas.DeleteShopNoContent{}, nil
}

// ListShopRevisions lists a Shop's Revisions.
func (h *Handler) ListShopRevisions(ctx context.Context, p oas.ListShopRevisionsParams) (oas.ListShopRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Prep.ShopRevisions(ctx, c, uuid.UUID(p.CampaignId), domain.ShopID(p.ShopId))
	if err != nil {
		return h.campaignProblem(ctx, "shop revisions", err), nil
	}
	return &oas.ListShopRevisionsOKHeaders{Response: revisionsOut(revs)}, nil
}

// RestoreShopRevision brings a Shop back to a Revision.
func (h *Handler) RestoreShopRevision(ctx context.Context, p oas.RestoreShopRevisionParams) (oas.RestoreShopRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.RestoreShop(ctx, c, uuid.UUID(p.CampaignId), domain.ShopID(p.ShopId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore shop", err), nil
	}
	return &oas.ShopHeaders{Response: shopOut(x)}, nil
}

// RerollStock generates a Shop's Stock afresh.
func (h *Handler) RerollStock(ctx context.Context, p oas.RerollStockParams) (oas.RerollStockRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	x, err := h.Prep.RerollStock(ctx, c, uuid.UUID(p.CampaignId), domain.ShopID(p.ShopId))
	if err != nil {
		return h.campaignProblem(ctx, "reroll stock", err), nil
	}
	return &oas.ShopHeaders{Response: shopOut(x)}, nil
}
