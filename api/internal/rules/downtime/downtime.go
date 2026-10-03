// Package downtime holds the rules of time between adventures: downtime days spent on crafting from
// Recipes, work, training and research, and how that time moves the Game Clock.
package downtime

import (
	"fmt"
	"maps"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
)

// DesignError is why a Recipe cannot be used, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// RefusedError is why downtime cannot be spent as asked, fit to show the Player.
type RefusedError string

func (e RefusedError) Error() string { return string(e) }

// Limits on a Recipe, and what the other activities earn and cost a day, in copper.
const (
	MaxName        = 80
	MaxSlug        = 80
	MaxQuantity    = 100
	MaxDays        = 365
	MaxCostCP      = 100_000_000
	MaxIngredients = 20
	wageCP         = 100
	trainingCP     = 500
	researchCP     = 100
	// TrainingDays is how long training in a tool or a language takes.
	TrainingDays = 50
)

// Ingredient is an Item a Recipe uses up, and how many.
type Ingredient struct {
	Item  string
	Count int
}

// Recipe makes Items from ingredients, with a tool, over days, at a cost in copper.
type Recipe struct {
	Name        string
	Makes       string
	Quantity    int
	Ingredients []Ingredient
	Tool        string
	Days        int
	CostCP      int
}

func checkIngredients(list []Ingredient) error {
	if len(list) > MaxIngredients {
		return DesignError("A Recipe has up to 20 ingredients.")
	}
	seen := map[string]bool{}
	for _, in := range list {
		if in.Item == "" || len(in.Item) > MaxSlug {
			return DesignError("Name each ingredient by its slug, up to 80 characters.")
		}
		if in.Count < 1 || in.Count > MaxQuantity {
			return DesignError("A Recipe uses 1 to 100 of each ingredient.")
		}
		if seen[in.Item] {
			return DesignError("List each ingredient once.")
		}
		seen[in.Item] = true
	}
	return nil
}

// CheckRecipe reports why a Recipe cannot be used, or nil.
func CheckRecipe(r Recipe) error {
	if name := strings.TrimSpace(r.Name); name == "" || len([]rune(name)) > MaxName {
		return DesignError("A Recipe needs a name of up to 80 characters.")
	}
	if r.Makes == "" || len(r.Makes) > MaxSlug {
		return DesignError("A Recipe names the Item it makes by its slug, up to 80 characters.")
	}
	if r.Quantity < 1 || r.Quantity > MaxQuantity {
		return DesignError("A Recipe makes 1 to 100 of its Item.")
	}
	if r.Days < 1 || r.Days > MaxDays {
		return DesignError("A Recipe takes 1 to 365 days.")
	}
	if r.CostCP < 0 || r.CostCP > MaxCostCP {
		return DesignError("A Recipe costs nothing or more, up to a million gold.")
	}
	if len(r.Tool) > MaxSlug {
		return DesignError("Name its tool by its slug, up to 80 characters.")
	}
	return checkIngredients(r.Ingredients)
}

// price writes an amount of copper as gold, silver and copper.
func price(cp int) string {
	var parts []string
	for _, c := range []struct {
		name  string
		value int
	}{{"gp", 100}, {"sp", 10}, {"cp", 1}} {
		if n := cp / c.value; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, c.name))
			cp -= n * c.value
		}
	}
	return strings.Join(parts, " ")
}

// Spend takes downtime days and coin together: the days left, and the purse after paying, or why not.
func Spend(have int, coins map[string]int, days, costCP int) (int, map[string]int, error) {
	if days < 1 {
		return have, coins, RefusedError("Spend at least one downtime day.")
	}
	if days > have {
		return have, coins, RefusedError(fmt.Sprintf("It takes %d downtime days; you have %d.", days, have))
	}
	left, ok := shops.Pay(coins, costCP)
	if !ok {
		return have, coins, RefusedError("It costs " + price(costCP) + ", which you cannot pay.")
	}
	return have - days, left, nil
}

// Bench is what a Character brings to crafting: the downtime days left, the coin carried, and how
// many of each Item is at hand.
type Bench struct {
	Days  int
	Coins map[string]int
	Items map[string]int
}

// Crafted is what crafting leaves: the days and coin left, and the new count of every Item it touched.
type Crafted struct {
	DaysLeft int
	Coins    map[string]int
	Items    map[string]int
}

// Craft makes a Recipe's Items. It takes the Recipe's days, coin and ingredients, needs its tool at
// hand and leaves it there. Whatever is short, nothing is taken.
func Craft(r Recipe, b Bench) (Crafted, error) {
	if r.Tool != "" && b.Items[r.Tool] < 1 {
		return Crafted{}, RefusedError("It needs a " + r.Tool + " at hand.")
	}
	items := map[string]int{}
	for _, in := range r.Ingredients {
		if have := b.Items[in.Item]; have < in.Count {
			return Crafted{}, RefusedError(fmt.Sprintf("It needs %d × %s; you have %d.", in.Count, in.Item, have))
		}
		items[in.Item] = b.Items[in.Item] - in.Count
	}
	left, coins, err := Spend(b.Days, b.Coins, r.Days, r.CostCP)
	if err != nil {
		return Crafted{}, err
	}
	// An ingredient that is also what the Recipe makes counts from what is left of it.
	made := b.Items[r.Makes]
	if after, used := items[r.Makes]; used {
		made = after
	}
	items[r.Makes] = made + r.Quantity
	return Crafted{DaysLeft: left, Coins: maps.Clone(coins), Items: items}, nil
}

// Wage is what days of work earn, in copper.
func Wage(days int) int { return days * wageCP }

// TrainingCost is what days of training cost, in copper.
func TrainingCost(days int) int { return days * trainingCP }

// ResearchCost is what days of research cost, in copper.
func ResearchCost(days int) int { return days * researchCP }

// Trained reports whether the days spent training in one thing are enough to have learned it.
func Trained(days int) bool { return days >= TrainingDays }

// ClockDays is how many days the Game Clock moves on when a Character has spent some days of the
// downtime: only the days nobody has yet lived through, since Characters spend theirs side by side.
func ClockDays(advanced, spent int) int {
	return max(0, spent-advanced)
}
