package app

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// StockReader is what generating Stock reads.
type StockReader interface {
	Settlements(ctx context.Context, campaign uuid.UUID) ([]domain.Settlement, error)
	LootTables(ctx context.Context, campaign uuid.UUID) ([]domain.LootTable, error)
	ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]domain.ItemPrice, error)
}

// Stock rolls a Shop's Loot Table for its Settlement's size and prices what its wealth stocks.
func Stock(ctx context.Context, r StockReader, campaign uuid.UUID, shop domain.Shop, src dice.Source) ([]domain.StockItem, error) {
	settlements, err := r.Settlements(ctx, campaign)
	if err != nil {
		return nil, err
	}
	place := settlements[slices.IndexFunc(settlements, func(x domain.Settlement) bool { return x.ID == shop.SettlementID })]
	tables, err := r.LootTables(ctx, campaign)
	if err != nil {
		return nil, err
	}
	rolled := domain.RollStock(src, shop, place.Size, tables)
	slugs := make([]string, 0, len(rolled))
	for slug := range rolled {
		slugs = append(slugs, slug)
	}
	prices, err := r.ItemPrices(ctx, campaign, slugs)
	if err != nil {
		return nil, err
	}
	return domain.PriceStock(rolled, place.Wealth, shop.MarkupPct, prices), nil
}
