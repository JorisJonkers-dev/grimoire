package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Settlements lists the Campaign's Settlements.
func (s *Store) Settlements(ctx context.Context, campaign uuid.UUID) ([]domain.Settlement, error) {
	return LoadSettlements(ctx, s.q, campaign)
}

// LoadSettlements reads the Campaign's Settlements.
func LoadSettlements(ctx context.Context, q *queries.Queries, campaign uuid.UUID) ([]domain.Settlement, error) {
	rows, err := q.ListSettlements(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Settlement, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Settlement{ID: domain.SettlementID(r.ID), Name: r.Name, Size: r.Size, Wealth: r.Wealth, LocationID: fromUUID(r.LocationID), UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

// SaveSettlement writes a Settlement.
func (s *Store) SaveSettlement(ctx context.Context, campaign uuid.UUID, x domain.Settlement, now time.Time) error {
	return s.q.SaveSettlement(ctx, queries.SaveSettlementParams{
		ID: uuid.UUID(x.ID), CampaignID: campaign, Name: x.Name, Size: x.Size, Wealth: x.Wealth, LocationID: optUUID(x.LocationID), Now: now,
	})
}

// DeleteSettlement removes a Settlement.
func (s *Store) DeleteSettlement(ctx context.Context, campaign uuid.UUID, id domain.SettlementID) (bool, error) {
	n, err := s.q.DeleteSettlement(ctx, queries.DeleteSettlementParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return n > 0, err
}

// RecordSettlement stores a Revision with the Settlement's snapshot.
func (s *Store) RecordSettlement(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, x domain.Settlement) error {
	revID, err := s.record(ctx, campaign, domain.EntitySettlement, uuid.UUID(x.ID), rev, c)
	if err != nil {
		return err
	}
	return s.q.InsertSettlementRevision(ctx, queries.InsertSettlementRevisionParams{RevisionID: revID, Name: x.Name, Size: x.Size, Wealth: x.Wealth, LocationID: optUUID(x.LocationID)})
}

// SettlementAt reads a Settlement as one of its Revisions recorded it.
func (s *Store) SettlementAt(ctx context.Context, campaign uuid.UUID, id domain.SettlementID, no int) (domain.Settlement, error) {
	r, err := s.q.GetSettlementRevision(ctx, queries.GetSettlementRevisionParams{CampaignID: campaign, EntityID: uuid.UUID(id), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.Settlement{}, notFound(err)
	}
	return domain.Settlement{ID: id, Name: r.Name, Size: r.Size, Wealth: r.Wealth, LocationID: fromUUID(r.LocationID)}, nil
}

func shopRow(r queries.PrepShop) domain.Shop {
	return domain.Shop{
		ID: domain.ShopID(r.ID), SettlementID: domain.SettlementID(r.SettlementID), Name: r.Name, Kind: r.Kind, OwnerID: fromUUID(r.OwnerNpcID), FactionID: fromUUID(r.FactionID),
		MarkupPct: int(r.MarkupPct), HaggleDC: int(r.HaggleDc), HagglePct: int(r.HagglePct), LootTable: lootRef(r.LootTableID), Restock: r.Restock,
		RestockDays: int(r.RestockDays.Int32), StockedDay: int(r.StockedDay), Stock: []domain.StockItem{}, UpdatedAt: r.UpdatedAt,
	}
}

// Shops lists the Campaign's Shops with their Stock.
func (s *Store) Shops(ctx context.Context, campaign uuid.UUID) ([]domain.Shop, error) {
	return LoadShops(ctx, s.q, campaign)
}

// LoadShops reads the Campaign's Shops with their Stock.
func LoadShops(ctx context.Context, q *queries.Queries, campaign uuid.UUID) ([]domain.Shop, error) {
	rows, err := q.CampaignShops(ctx, campaign)
	if err != nil {
		return nil, err
	}
	stock, err := q.CampaignShopStock(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Shop, 0, len(rows))
	for _, r := range rows {
		x := shopRow(r)
		for _, k := range stock {
			if k.ShopID == r.ID {
				x.Stock = append(x.Stock, domain.StockItem{Slug: k.ItemSlug, Quantity: int(k.Quantity), PriceCP: int(k.PriceCp)})
			}
		}
		out = append(out, x)
	}
	return out, nil
}

func optInt(n int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(n), Valid: n > 0} //nolint:gosec // bounded by the rules
}

// SaveShop writes a Shop's fields.
//
//nolint:gosec // percentages and days are bounded by the rules
func (s *Store) SaveShop(ctx context.Context, x domain.Shop, now time.Time) error {
	return s.q.SaveShop(ctx, queries.SaveShopParams{
		ID: uuid.UUID(x.ID), SettlementID: uuid.UUID(x.SettlementID), Name: x.Name, Kind: x.Kind, OwnerNpcID: optUUID(x.OwnerID), FactionID: optUUID(x.FactionID),
		MarkupPct: int32(x.MarkupPct), HaggleDc: int32(x.HaggleDC), HagglePct: int32(x.HagglePct), LootTableID: lootUUID(x.LootTable),
		Restock: x.Restock, RestockDays: optInt(x.RestockDays), StockedDay: int32(x.StockedDay), Now: now,
	})
}

// SetStock replaces a Shop's Stock and records the day it was stocked.
func (s *Store) SetStock(ctx context.Context, id domain.ShopID, stock []domain.StockItem, day int) error {
	return WriteStock(ctx, s.q, id, stock, day)
}

// WriteStock replaces a Shop's Stock and records the day it was stocked.
//
//nolint:gosec // counts, prices and days are bounded by the rules
func WriteStock(ctx context.Context, q *queries.Queries, id domain.ShopID, stock []domain.StockItem, day int) error {
	sid := uuid.UUID(id)
	if err := q.ClearShopStock(ctx, sid); err != nil {
		return err
	}
	for _, k := range stock {
		if err := q.SetShopStock(ctx, queries.SetShopStockParams{ShopID: sid, ItemSlug: k.Slug, Quantity: int32(k.Quantity), PriceCp: int32(k.PriceCP)}); err != nil {
			return err
		}
	}
	return q.SetShopStockedDay(ctx, queries.SetShopStockedDayParams{ID: sid, StockedDay: int32(day)})
}

// DeleteShop removes a Shop.
func (s *Store) DeleteShop(ctx context.Context, campaign uuid.UUID, id domain.ShopID) (bool, error) {
	n, err := s.q.DeleteShop(ctx, queries.DeleteShopParams{CampaignID: campaign, ID: uuid.UUID(id)})
	return n > 0, err
}

// RecordShop stores a Revision with the Shop's snapshot, Stock included.
func (s *Store) RecordShop(ctx context.Context, campaign uuid.UUID, rev campaigndomain.Revision, c caller.Caller, x domain.Shop) error {
	revID, err := s.record(ctx, campaign, domain.EntityShop, uuid.UUID(x.ID), rev, c)
	if err != nil {
		return err
	}
	return ShopSnapshot(ctx, s.q, revID, x)
}

// ShopSnapshot stores a Shop and its Stock against a Revision.
//
//nolint:gosec // percentages, counts and prices are bounded by the rules
func ShopSnapshot(ctx context.Context, q *queries.Queries, revID uuid.UUID, x domain.Shop) error {
	if err := q.InsertShopRevision(ctx, queries.InsertShopRevisionParams{
		RevisionID: revID, SettlementID: uuid.UUID(x.SettlementID), Name: x.Name, Kind: x.Kind, OwnerNpcID: optUUID(x.OwnerID), FactionID: optUUID(x.FactionID), MarkupPct: int32(x.MarkupPct),
		HaggleDc: int32(x.HaggleDC), HagglePct: int32(x.HagglePct), LootTableID: lootUUID(x.LootTable), Restock: x.Restock, RestockDays: optInt(x.RestockDays),
		StockedDay: int32(x.StockedDay),
	}); err != nil {
		return err
	}
	for _, k := range x.Stock {
		if err := q.InsertShopRevisionStock(ctx, queries.InsertShopRevisionStockParams{RevisionID: revID, ItemSlug: k.Slug, Quantity: int32(k.Quantity), PriceCp: int32(k.PriceCP)}); err != nil {
			return err
		}
	}
	return nil
}

// ShopAt reads a Shop and its Stock as one of its Revisions recorded them.
func (s *Store) ShopAt(ctx context.Context, campaign uuid.UUID, id domain.ShopID, no int) (domain.Shop, error) {
	r, err := s.q.GetShopRevision(ctx, queries.GetShopRevisionParams{CampaignID: campaign, EntityID: uuid.UUID(id), RevisionNo: int32(no)}) //nolint:gosec // bounded by the API
	if err != nil {
		return domain.Shop{}, notFound(err)
	}
	rows, err := s.q.ShopRevisionStock(ctx, r.ID)
	if err != nil {
		return domain.Shop{}, err
	}
	x := domain.Shop{
		ID: id, SettlementID: domain.SettlementID(r.SettlementID), Name: r.Name, Kind: r.Kind, OwnerID: fromUUID(r.OwnerNpcID), FactionID: fromUUID(r.FactionID), MarkupPct: int(r.MarkupPct),
		HaggleDC: int(r.HaggleDc), HagglePct: int(r.HagglePct), LootTable: lootRef(r.LootTableID), Restock: r.Restock, RestockDays: int(r.RestockDays.Int32),
		StockedDay: int(r.StockedDay), Stock: []domain.StockItem{},
	}
	for _, k := range rows {
		x.Stock = append(x.Stock, domain.StockItem{Slug: k.ItemSlug, Quantity: int(k.Quantity), PriceCP: int(k.PriceCp)})
	}
	return x, nil
}

// FactionExists reports whether a Faction belongs to the Campaign.
func (s *Store) FactionExists(ctx context.Context, campaign, id uuid.UUID) (bool, error) {
	return s.q.FactionExists(ctx, queries.FactionExistsParams{CampaignID: campaign, ID: id})
}

// NpcExists reports whether an NPC belongs to the Campaign.
func (s *Store) NpcExists(ctx context.Context, campaign, id uuid.UUID) (bool, error) {
	_, err := s.q.CampaignNpc(ctx, queries.CampaignNpcParams{CampaignID: campaign, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// ItemPrices reads what items cost.
func (s *Store) ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]domain.ItemPrice, error) {
	return LoadItemPrices(ctx, s.q, campaign, slugs)
}

// LoadItemPrices reads what items are called, cost and weigh.
func LoadItemPrices(ctx context.Context, q *queries.Queries, campaign uuid.UUID, slugs []string) (map[string]domain.ItemPrice, error) {
	out := map[string]domain.ItemPrice{}
	if len(slugs) == 0 {
		return out, nil
	}
	rows, err := q.ItemPrices(ctx, queries.ItemPricesParams{Slugs: slugs, CampaignID: campaign})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.Slug] = domain.ItemPrice{Name: r.Name, CostCP: int(r.CostCp), Magic: r.Magic, Rarity: r.Rarity, WeightLb: r.WeightLb}
	}
	return out, nil
}

// GameDay reads the Campaign's in-game day.
func (s *Store) GameDay(ctx context.Context, campaign uuid.UUID) (int, error) {
	d, err := s.q.GameDay(ctx, campaign)
	return int(d), err
}

// RecordRestock records a Shop's restock as a Revision with its new Stock.
func RecordRestock(ctx context.Context, q *queries.Queries, campaign uuid.UUID, x domain.Shop, author string, c caller.Caller, now time.Time) error {
	id := uuid.UUID(x.ID)
	no, err := q.NextRevisionNo(ctx, queries.NextRevisionNoParams{EntityType: domain.EntityShop, EntityID: id})
	if err != nil {
		return err
	}
	revID, err := q.InsertRevision(ctx, queries.InsertRevisionParams{
		CampaignID: campaign, EntityType: domain.EntityShop, EntityID: id, RevisionNo: no, Action: "update",
		CallerSubject: c.Subject, CallerName: author, Origin: string(c.Origin), Client: c.Client, Now: now,
	})
	if err != nil {
		return err
	}
	return ShopSnapshot(ctx, q, revID, x)
}
