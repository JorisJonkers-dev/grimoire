// Package shops holds the trading rules: how much Stock a Settlement's size allows, what its wealth
// lets a Shop carry, prices in copper, paying with change, and haggling.
package shops

import "slices"

// Settlement sizes, smallest first.
func Sizes() []string {
	return []string{"hamlet", "village", "town", "city"}
}

// Wealth tiers, poorest first.
func Wealths() []string {
	return []string{"poor", "modest", "comfortable", "wealthy"}
}

// StockRolls is how many times a Shop's Loot Table is rolled for its Stock: once per size step.
func StockRolls(size string) int {
	return slices.Index(Sizes(), size) + 1
}

// PriceCap is the dearest item a Settlement of this wealth stocks, in copper; 0 means no limit.
func PriceCap(wealth string) int {
	return [...]int{10_00, 100_00, 1000_00, 0}[slices.Index(Wealths(), wealth)]
}

// Stocks reports whether a Settlement of this wealth stocks an item of this base price.
func Stocks(wealth string, baseCP int) bool {
	limit := PriceCap(wealth)
	return limit == 0 || baseCP <= limit
}

// RarityCP is a magic item's value by rarity in the 2024 rules, in copper; 0 for an unknown rarity.
func RarityCP(rarity string) int {
	return map[string]int{"common": 100_00, "uncommon": 400_00, "rare": 4000_00, "very rare": 40000_00, "legendary": 200000_00}[rarity]
}

// Price is what a Shop asks: the base price with its markup and any haggled adjustment, in percent,
// never below one copper.
func Price(baseCP, markupPct, adjustPct int) int {
	return max(1, baseCP*(100+markupPct)*(100+adjustPct)/10000)
}

// SellPrice is what a Shop pays for an item: half its base price.
func SellPrice(baseCP int) int {
	return baseCP / 2
}

// Coin values in copper, dearest first.
func coinValues() []struct {
	coin  string
	value int
} {
	return []struct {
		coin  string
		value int
	}{{"pp", 1000}, {"gp", 100}, {"sp", 10}, {"cp", 1}}
}

// Worth is a purse's value in copper.
func Worth(purse map[string]int) int {
	total := purse["ep"] * 50
	for _, c := range coinValues() {
		total += purse[c.coin] * c.value
	}
	return total
}

// Pay takes a price in copper from a purse and gives change in the fewest coins; ok is false when the
// purse cannot cover it.
func Pay(purse map[string]int, priceCP int) (map[string]int, bool) {
	left := Worth(purse) - priceCP
	if left < 0 {
		return purse, false
	}
	return Coins(left), true
}

// Coins is an amount of copper in the fewest platinum, gold, silver and copper pieces.
func Coins(cp int) map[string]int {
	out := map[string]int{}
	for _, c := range coinValues() {
		if n := cp / c.value; n > 0 {
			out[c.coin] = n
			cp -= n * c.value
		}
	}
	return out
}

// Haggle turns a haggling roll against the Shop's DC into a price adjustment in percent, within its
// bound: beating the DC takes 10% off, beating it by 10 or more 20%; missing it by 5 or more adds 10%.
func Haggle(total, dc, boundPct int) int {
	adjust := 0
	switch {
	case total >= dc+10:
		adjust = -20
	case total >= dc:
		adjust = -10
	case total <= dc-5:
		adjust = 10
	}
	return max(-boundPct, min(boundPct, adjust))
}
