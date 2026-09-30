package rules

import "sort"

// Method is how base ability scores were generated.
type Method string

// Generation methods.
const (
	StandardArray Method = "standard-array"
	PointBuy      Method = "point-buy"
	Rolled        Method = "rolled"
)

// PointBuyBudget is the points a player may spend.
const PointBuyBudget = 27

// PointBuyCost is the cost of one score under point buy, and whether the score is buyable.
func PointBuyCost(score int) (int, bool) {
	switch {
	case score < 8 || score > 15:
		return 0, false
	case score <= 13:
		return score - 8, true
	default:
		return 5 + (score-13)*2, true
	}
}

// ValidateBase checks base scores (before origin bonuses) against the generation method.
func ValidateBase(method Method, base map[Ability]int) error {
	if len(base) != 6 {
		return violation("all six abilities need a score")
	}
	for a := range base {
		if !a.Valid() {
			return violation("%q is not an ability", a)
		}
	}
	switch method {
	case StandardArray:
		return standardArray(base)
	case PointBuy:
		return pointBuy(base)
	case Rolled:
		for a, s := range base {
			if s < 3 || s > 18 {
				return violation("a rolled %s of %d is impossible on 4d6 drop lowest", a, s)
			}
		}
		return nil
	default:
		return violation("unknown method %q", method)
	}
}

func standardArray(base map[Ability]int) error {
	got := make([]int, 0, 6)
	for _, s := range base {
		got = append(got, s)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(got)))
	for i, want := range []int{15, 14, 13, 12, 10, 8} {
		if got[i] != want {
			return violation("the standard array is 15, 14, 13, 12, 10 and 8, each used once")
		}
	}
	return nil
}

func pointBuy(base map[Ability]int) error {
	spent := 0
	for a, s := range base {
		cost, ok := PointBuyCost(s)
		if !ok {
			return violation("point buy scores run from 8 to 15; %s is %d", a, s)
		}
		spent += cost
	}
	if spent > PointBuyBudget {
		return violation("point buy allows %d points; these cost %d", PointBuyBudget, spent)
	}
	return nil
}

// ValidateOriginBonuses checks origin ability increases: +2 and +1, or +1 to three abilities.
// When allowed is non-empty (the 2024 background list), every increase must be to one of them.
func ValidateOriginBonuses(bonus map[Ability]int, allowed []Ability) error {
	counts := map[int]int{}
	for a, b := range bonus {
		if !a.Valid() {
			return violation("%q is not an ability", a)
		}
		if len(allowed) > 0 && !contains(allowed, a) {
			return violation("your background does not raise %s", a)
		}
		counts[b]++
	}
	switch {
	case len(bonus) == 2 && counts[2] == 1 && counts[1] == 1:
		return nil
	case len(bonus) == 3 && counts[1] == 3:
		return nil
	default:
		return violation("origin increases are +2 and +1, or +1 to three different abilities")
	}
}

func contains[T comparable](list []T, x T) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// FinalScores adds origin bonuses to base scores, refusing any score above 20.
func FinalScores(base, bonus map[Ability]int) (map[Ability]int, error) {
	out := make(map[Ability]int, len(base))
	for a, s := range base {
		out[a] = s + bonus[a]
		if out[a] > 20 {
			return nil, violation("%s cannot exceed 20", a)
		}
	}
	return out, nil
}
