package downtime_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
)

func potion() downtime.Recipe {
	return downtime.Recipe{
		Name: "Potion of Healing", Makes: "potion-of-healing", Quantity: 2, Tool: "herbalism-kit", Days: 3, CostCP: 2500,
		Ingredients: []downtime.Ingredient{{Item: "healing-herb", Count: 2}, {Item: "vial", Count: 1}},
	}
}

// A Recipe makes something, from ingredients or from coin and time alone, within bounds.
func TestCheckingARecipe(t *testing.T) {
	t.Parallel()
	if err := downtime.CheckRecipe(potion()); err != nil {
		t.Fatalf("a sound recipe: %v", err)
	}
	change := func(f func(r *downtime.Recipe)) downtime.Recipe {
		r := potion()
		r.Ingredients = append([]downtime.Ingredient{}, r.Ingredients...)
		f(&r)
		return r
	}
	twenty := make([]downtime.Ingredient, 20)
	for i := range twenty {
		twenty[i] = downtime.Ingredient{Item: "herb-" + string(rune('a'+i)), Count: 1}
	}
	for name, r := range map[string]downtime.Recipe{
		"nothing but time": change(func(r *downtime.Recipe) { r.Ingredients, r.Tool, r.CostCP = nil, "", 0 }),
		"the longest name": change(func(r *downtime.Recipe) { r.Name = strings.Repeat("a", 80) }),
		"the longest slugs": change(func(r *downtime.Recipe) {
			r.Makes, r.Tool, r.Ingredients[0].Item = strings.Repeat("a", 80), strings.Repeat("b", 80), strings.Repeat("c", 80)
		}),
		"one made in a day":  change(func(r *downtime.Recipe) { r.Quantity, r.Days = 1, 1 }),
		"a hundred made":     change(func(r *downtime.Recipe) { r.Quantity = 100 }),
		"a year of work":     change(func(r *downtime.Recipe) { r.Days = 365 }),
		"the dearest":        change(func(r *downtime.Recipe) { r.CostCP = 100_000_000 }),
		"twenty ingredients": change(func(r *downtime.Recipe) { r.Ingredients = twenty }),
		"a hundred of one":   change(func(r *downtime.Recipe) { r.Ingredients[0].Count = 100 }),
	} {
		if err := downtime.CheckRecipe(r); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for want, r := range map[string]downtime.Recipe{
		"needs a name":              change(func(r *downtime.Recipe) { r.Name = "  " }),
		"name of up to 80":          change(func(r *downtime.Recipe) { r.Name = strings.Repeat("a", 81) }),
		"names the Item it makes":   change(func(r *downtime.Recipe) { r.Makes = "" }),
		"Item it makes by its slug": change(func(r *downtime.Recipe) { r.Makes = strings.Repeat("a", 81) }),
		"makes 1 to 100":            change(func(r *downtime.Recipe) { r.Quantity = 0 }),
		"makes 1 to 100 ":           change(func(r *downtime.Recipe) { r.Quantity = 101 }),
		"takes 1 to 365 days":       change(func(r *downtime.Recipe) { r.Days = 0 }),
		"takes 1 to 365 days ":      change(func(r *downtime.Recipe) { r.Days = 366 }),
		"costs nothing or more":     change(func(r *downtime.Recipe) { r.CostCP = -1 }),
		"costs nothing or more ":    change(func(r *downtime.Recipe) { r.CostCP = 100_000_001 }),
		"tool by its slug":          change(func(r *downtime.Recipe) { r.Tool = strings.Repeat("a", 81) }),
		"up to 20 ingredients": change(func(r *downtime.Recipe) {
			r.Ingredients = slices.Concat(twenty, []downtime.Ingredient{{Item: "one-more", Count: 1}})
		}),
		"each ingredient by its slug":  change(func(r *downtime.Recipe) { r.Ingredients[1].Item = "" }),
		"each ingredient by its slug ": change(func(r *downtime.Recipe) { r.Ingredients[1].Item = strings.Repeat("a", 81) }),
		"1 to 100 of each":             change(func(r *downtime.Recipe) { r.Ingredients[1].Count = 0 }),
		"1 to 100 of each ":            change(func(r *downtime.Recipe) { r.Ingredients[1].Count = 101 }),
		"each ingredient once":         change(func(r *downtime.Recipe) { r.Ingredients[1].Item = "healing-herb" }),
	} {
		var bad downtime.DesignError
		if err := downtime.CheckRecipe(r); !errors.As(err, &bad) || !strings.Contains(err.Error(), strings.TrimSpace(want)) {
			t.Errorf("%q: %v", want, err)
		}
	}
}

// Crafting takes the Recipe's days, coin and ingredients, needs its tool at hand and leaves it there,
// and gives what the Recipe makes. Whatever is short, nothing is taken.
func TestCrafting(t *testing.T) {
	t.Parallel()
	bench := func() downtime.Bench {
		return downtime.Bench{Days: 5, Coins: map[string]int{"gp": 30, "sp": 4}, Items: map[string]int{"healing-herb": 3, "vial": 1, "herbalism-kit": 1, "potion-of-healing": 1, "rope": 2}}
	}
	got, err := downtime.Craft(potion(), bench())
	if err != nil {
		t.Fatal(err)
	}
	if got.DaysLeft != 2 || !maps.Equal(got.Items, map[string]int{"healing-herb": 1, "vial": 0, "potion-of-healing": 3}) {
		t.Fatalf("crafted = %+v", got)
	}
	// 30 gp 4 sp is 3040 cp; less 2500 leaves 540: 5 gp 4 sp.
	if !maps.Equal(got.Coins, map[string]int{"gp": 5, "sp": 4}) {
		t.Fatalf("coins left = %v", got.Coins)
	}
	with := func(f func(b *downtime.Bench)) downtime.Bench {
		b := bench()
		f(&b)
		return b
	}
	for want, b := range map[string]downtime.Bench{
		"It takes 3 downtime days; you have 2.":  with(func(b *downtime.Bench) { b.Days = 2 }),
		"It needs a herbalism-kit at hand.":      with(func(b *downtime.Bench) { delete(b.Items, "herbalism-kit") }),
		"It needs 2 × healing-herb; you have 1.": with(func(b *downtime.Bench) { b.Items["healing-herb"] = 1 }),
		"It needs 1 × vial; you have 0.":         with(func(b *downtime.Bench) { delete(b.Items, "vial") }),
		"It costs 25 gp, which you cannot pay.":  with(func(b *downtime.Bench) { b.Coins = map[string]int{"gp": 24, "sp": 9, "cp": 9} }),
	} {
		var refused downtime.RefusedError
		if _, err := downtime.Craft(potion(), b); !errors.As(err, &refused) || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}
	// Exactly enough of everything is enough, and what was given is not changed.
	exact := downtime.Bench{Days: 3, Coins: map[string]int{"gp": 25}, Items: map[string]int{"healing-herb": 2, "vial": 1, "herbalism-kit": 1}}
	got, err = downtime.Craft(potion(), exact)
	if err != nil || got.DaysLeft != 0 || got.Items["potion-of-healing"] != 2 || got.Items["healing-herb"] != 0 || len(got.Coins) != 0 {
		t.Fatalf("with exactly enough = %+v %v", got, err)
	}
	if exact.Items["healing-herb"] != 2 || exact.Coins["gp"] != 25 {
		t.Fatalf("the bench was changed: %+v", exact)
	}
	// A Recipe of time alone asks for no tool, no ingredient and no coin; a price in odd coppers is named so.
	plain := downtime.Recipe{Name: "Whittling", Makes: "wooden-spoon", Quantity: 1, Days: 1}
	if got, err := downtime.Craft(plain, downtime.Bench{Days: 1}); err != nil || got.Items["wooden-spoon"] != 1 || got.DaysLeft != 0 {
		t.Fatalf("a Recipe of time alone = %+v %v", got, err)
	}
	// What a Recipe makes may be one of its ingredients: a starter feeds itself.
	starter := downtime.Recipe{Name: "Feed the starter", Makes: "starter", Quantity: 3, Days: 1, Ingredients: []downtime.Ingredient{{Item: "starter", Count: 1}}}
	if got, err := downtime.Craft(starter, downtime.Bench{Days: 1, Items: map[string]int{"starter": 2}}); err != nil || got.Items["starter"] != 4 {
		t.Fatalf("a Recipe that feeds itself = %+v %v", got, err)
	}
	odd := plain
	odd.CostCP = 1234
	if _, err := downtime.Craft(odd, downtime.Bench{Days: 1}); err == nil || err.Error() != "It costs 12 gp 3 sp 4 cp, which you cannot pay." {
		t.Fatalf("an odd price = %v", err)
	}
}

// Work earns a wage for each day; training and research cost coin for each day, and training is done
// after fifty days.
func TestWorkTrainingAndResearch(t *testing.T) {
	t.Parallel()
	if downtime.Wage(0) != 0 || downtime.Wage(1) != 100 || downtime.Wage(7) != 700 {
		t.Errorf("wages = %d %d %d", downtime.Wage(0), downtime.Wage(1), downtime.Wage(7))
	}
	if downtime.TrainingCost(1) != 500 || downtime.TrainingCost(10) != 5000 || downtime.ResearchCost(1) != 100 || downtime.ResearchCost(4) != 400 {
		t.Errorf("costs = %d %d %d %d", downtime.TrainingCost(1), downtime.TrainingCost(10), downtime.ResearchCost(1), downtime.ResearchCost(4))
	}
	for days, want := range map[int]bool{0: false, 49: false, 50: true, 51: true} {
		if got := downtime.Trained(days); got != want {
			t.Errorf("trained after %d days = %v", days, got)
		}
	}
	// Spending takes days and coin together, or neither.
	left, coins, err := downtime.Spend(5, map[string]int{"gp": 20}, 3, 1500)
	if err != nil || left != 2 || !maps.Equal(coins, map[string]int{"gp": 5}) {
		t.Fatalf("spend = %d %v %v", left, coins, err)
	}
	if left, coins, err := downtime.Spend(3, map[string]int{"gp": 15}, 3, 1500); err != nil || left != 0 || len(coins) != 0 {
		t.Fatalf("spending it all = %d %v %v", left, coins, err)
	}
	for want, c := range map[string]struct{ have, days, cost int }{
		"It takes 4 downtime days; you have 3.": {3, 4, 0},
		"It costs 16 gp, which you cannot pay.": {5, 3, 1600},
		"Spend at least one downtime day.":      {5, 0, 0},
	} {
		if _, _, err := downtime.Spend(c.have, map[string]int{"gp": 15}, c.days, c.cost); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}

// The Game Clock moves on only for days nobody has yet lived through: Characters spend their downtime
// side by side.
func TestDowntimeMovesTheClockOnce(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ advanced, spent, want int }{{0, 3, 3}, {3, 3, 0}, {3, 2, 0}, {3, 5, 2}, {0, 0, 0}} {
		if got := downtime.ClockDays(c.advanced, c.spent); got != c.want {
			t.Errorf("clock already %d days on, a Character %d days in: %d", c.advanced, c.spent, got)
		}
	}
}
