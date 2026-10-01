package shops_test

import (
	"maps"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
)

func TestSizeAndWealthShapeTheStock(t *testing.T) {
	for size, want := range map[string]int{"hamlet": 1, "village": 2, "town": 3, "city": 4, "metropolis": 0} {
		if got := shops.StockRolls(size); got != want {
			t.Errorf("StockRolls(%s) = %d, want %d", size, got, want)
		}
	}
	for wealth, want := range map[string]int{"poor": 1000, "modest": 10000, "comfortable": 100000, "wealthy": 0} {
		if got := shops.PriceCap(wealth); got != want {
			t.Errorf("PriceCap(%s) = %d, want %d", wealth, got, want)
		}
	}
	if !shops.Stocks("poor", 1000) || shops.Stocks("poor", 1001) || !shops.Stocks("wealthy", 1_000_000_00) {
		t.Error("stocks")
	}
	if len(shops.Sizes()) != 4 || len(shops.Wealths()) != 4 {
		t.Error("tiers")
	}
	for rarity, want := range map[string]int{"common": 10000, "uncommon": 40000, "rare": 400000, "very rare": 4000000, "legendary": 20000000, "artifact": 0} {
		if got := shops.RarityCP(rarity); got != want {
			t.Errorf("RarityCP(%s) = %d, want %d", rarity, got, want)
		}
	}
}

func TestPricesMarkupsAndSelling(t *testing.T) {
	for _, c := range []struct{ base, markup, adjust, want int }{
		{1000, 0, 0, 1000}, {1000, 50, 0, 1500}, {1000, 50, -10, 1350}, {1000, 0, 10, 1100}, {1, 0, -20, 1}, {0, 100, 0, 1}, {333, 20, 0, 399},
	} {
		if got := shops.Price(c.base, c.markup, c.adjust); got != c.want {
			t.Errorf("Price(%d, %d, %d) = %d, want %d", c.base, c.markup, c.adjust, got, c.want)
		}
	}
	if shops.SellPrice(1001) != 500 || shops.SellPrice(1) != 0 {
		t.Error("sell price")
	}
}

func TestPayingGivesChangeInTheFewestCoins(t *testing.T) {
	purse := map[string]int{"gp": 3, "sp": 5, "cp": 2, "ep": 1, "pp": 1}
	if shops.Worth(purse) != 300+50+2+50+1000 {
		t.Fatalf("worth = %d", shops.Worth(purse))
	}
	left, ok := shops.Pay(purse, 1201)
	if !ok || !maps.Equal(left, map[string]int{"gp": 2, "cp": 1}) {
		t.Fatalf("pay = %v %v", left, ok)
	}
	if left, ok := shops.Pay(purse, 1403); ok || !maps.Equal(left, purse) {
		t.Fatalf("too dear = %v %v", left, ok)
	}
	if left, ok := shops.Pay(purse, 1402); !ok || len(left) != 0 {
		t.Fatalf("exact = %v %v", left, ok)
	}
	if !maps.Equal(shops.Coins(1234), map[string]int{"pp": 1, "gp": 2, "sp": 3, "cp": 4}) {
		t.Fatalf("coins = %v", shops.Coins(1234))
	}
}

func TestHagglingAdjustsWithinTheBound(t *testing.T) {
	for _, c := range []struct{ total, dc, bound, want int }{
		{25, 15, 20, -20}, {24, 15, 20, -10}, {15, 15, 20, -10}, {14, 15, 20, 0}, {11, 15, 20, 0}, {10, 15, 20, 10}, {25, 15, 5, -5}, {10, 15, 5, 5}, {30, 15, 0, 0},
	} {
		if got := shops.Haggle(c.total, c.dc, c.bound); got != c.want {
			t.Errorf("Haggle(%d, %d, %d) = %d, want %d", c.total, c.dc, c.bound, got, c.want)
		}
	}
}
