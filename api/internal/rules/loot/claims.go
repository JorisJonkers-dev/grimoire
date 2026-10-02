package loot

import (
	"cmp"
	"slices"
)

// Claim choices: Need beats Greed.
const (
	Need  = "need"
	Greed = "greed"
)

// Claim is one claimant's call on an item in a loot pile, with the d20 rolled for it and when it came
// (Order: earlier is smaller).
type Claim struct {
	Claimant string
	Choice   string
	Roll     int
	Order    int
}

// Rank orders the claims that count for one item: only Need claims when there are any, then the higher
// roll, the earlier claim and finally the claimant, so conflicting claims always settle the same way.
func Rank(claims []Claim) []Claim {
	if slices.ContainsFunc(claims, func(c Claim) bool { return c.Choice == Need }) {
		claims = slices.DeleteFunc(slices.Clone(claims), func(c Claim) bool { return c.Choice != Need })
	} else {
		claims = slices.Clone(claims)
	}
	slices.SortFunc(claims, func(a, b Claim) int {
		return cmp.Or(cmp.Compare(b.Roll, a.Roll), cmp.Compare(a.Order, b.Order), cmp.Compare(a.Claimant, b.Claimant))
	})
	return claims
}

// Share deals n of an item one at a time to the ranked claimants, the best first.
func Share(n int, ranked []Claim) map[string]int {
	out := map[string]int{}
	if len(ranked) == 0 {
		return out
	}
	for i := range n {
		out[ranked[i%len(ranked)].Claimant]++
	}
	return out
}

// changing is one coin and the next smaller coin worth sharing it as.
type changing struct {
	coin, into string
	rate       int
}

// change is how each coin breaks down, dearest first: a gold piece into silver, never into electrum.
func change() []changing {
	return []changing{{"pp", "gp", 10}, {"gp", "sp", 10}, {"ep", "sp", 5}, {"sp", "cp", 10}, {"cp", "", 0}}
}

// Split shares coins evenly between k: what each gets, and what is left over. A remainder is changed
// into the next smaller coin and shared again; only copper that cannot be shared is left.
func Split(coins map[string]int, k int) (map[string]int, map[string]int) {
	each, left := map[string]int{}, map[string]int{}
	carry := map[string]int{}
	for _, c := range change() {
		n := coins[c.coin] + carry[c.coin]
		if n == 0 {
			continue
		}
		if k < 1 {
			left[c.coin] = coins[c.coin]
			continue
		}
		if n/k > 0 {
			each[c.coin] = n / k
		}
		switch rem := n % k; {
		case rem == 0:
		case c.into == "":
			left[c.coin] = rem
		default:
			carry[c.into] += rem * c.rate
		}
	}
	return each, left
}
