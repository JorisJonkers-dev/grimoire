package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
)

// SettlementID identifies a Settlement.
type SettlementID uuid.UUID

// ShopID identifies a Shop.
type ShopID uuid.UUID

// Settlement is a named inhabited place with a size and a wealth tier, perhaps at a location on a world map.
type Settlement struct {
	ID         SettlementID
	Name       string
	Size       string
	Wealth     string
	LocationID *uuid.UUID
	UpdatedAt  time.Time
}

// Restock rules of a Shop.
const (
	RestockNever    = "never"
	RestockLongRest = "long_rest"
	RestockDays     = "days"
)

// StockItem is how many of an item a Shop sells and its asking price in copper.
type StockItem struct {
	Slug     string
	Quantity int
	PriceCP  int
}

// Shop is a trader in a Settlement: its trade, owner, markup, how far it haggles, where its Stock
// comes from and when it restocks. StockedDay is the in-game day it last restocked.
type Shop struct {
	ID           ShopID
	SettlementID SettlementID
	Name         string
	Kind         string
	OwnerID      *uuid.UUID
	// FactionID is the Faction the Shop belongs to, if any: it prices by that Faction's Standing.
	FactionID   *uuid.UUID
	MarkupPct   int
	HaggleDC    int
	HagglePct   int
	LootTable   *LootTableID
	Restock     string
	RestockDays int
	StockedDay  int
	Stock       []StockItem
	UpdatedAt   time.Time
}

// Due reports whether the Shop restocks now: after a long rest, or once enough days have passed.
func (s Shop) Due(longRest bool, day int) bool {
	switch s.Restock {
	case RestockLongRest:
		return longRest
	case RestockDays:
		return day-s.StockedDay >= s.RestockDays
	}
	return false
}

// ItemPrice is what an item is called, costs and weighs; a magic item without a price is valued by rarity.
type ItemPrice struct {
	Name     string
	CostCP   int
	Magic    bool
	Rarity   string
	WeightLb float64
}

// BaseCP is the item's base price in copper.
func (p ItemPrice) BaseCP() int {
	if p.CostCP == 0 && p.Magic {
		return shops.RarityCP(p.Rarity)
	}
	return p.CostCP
}

// LootRules turns Loot Tables into the rules' tables, keyed by id.
func LootRules(tables []LootTable) map[string]loot.Table {
	out := map[string]loot.Table{}
	for _, t := range tables {
		rt := loot.Table{Rolls: t.Rolls, Entries: []loot.Entry{}}
		for _, e := range t.Entries {
			amount, _ := loot.ParseAmount(e.Amount)
			entry := loot.Entry{Weight: e.Weight, Kind: e.Kind, Slug: e.Item, Coin: e.Coin, Amount: amount, Table: ""}
			if e.Table != nil {
				entry.Table = uuid.UUID(*e.Table).String()
			}
			rt.Entries = append(rt.Entries, entry)
		}
		out[uuid.UUID(t.ID).String()] = rt
	}
	return out
}

// RollStock rolls a Shop's Loot Table once per size step of its Settlement, keeping items and dropping coins.
func RollStock(src dice.Source, shop Shop, size string, tables []LootTable) map[string]int {
	out := map[string]int{}
	if shop.LootTable == nil {
		return out
	}
	rules := LootRules(tables)
	for range shops.StockRolls(size) {
		for _, d := range loot.Roll(src, uuid.UUID(*shop.LootTable).String(), rules) {
			if d.Slug != "" {
				out[d.Slug] += d.Count
			}
		}
	}
	return out
}

// PriceStock keeps the rolled items the Settlement's wealth stocks and prices them with the Shop's markup.
func PriceStock(rolled map[string]int, wealth string, markupPct int, prices map[string]ItemPrice) []StockItem {
	out := []StockItem{}
	for slug, n := range rolled {
		p, ok := prices[slug]
		if !ok || !shops.Stocks(wealth, p.BaseCP()) {
			continue
		}
		out = append(out, StockItem{Slug: slug, Quantity: n, PriceCP: shops.Price(p.BaseCP(), markupPct, 0)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// Revisioned entity types of settlements and shops.
const (
	EntitySettlement = "settlement"
	EntityShop       = "shop"
)
