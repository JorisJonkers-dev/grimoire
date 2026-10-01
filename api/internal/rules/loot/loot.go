// Package loot holds the treasure rules: amounts such as "2d6x10", Loot Tables rolled with nested
// tables, and the 2024 carrying capacity.
package loot

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/encounters"
)

// ErrAmount is returned for an amount this package does not read.
var ErrAmount = errors.New("loot: bad amount")

// Amount is how many of something a drop holds: dice plus a flat number, times a multiplier.
type Amount struct {
	Dice  int
	Faces int
	Flat  int
	Times int
}

func amountPattern() *regexp.Regexp {
	return regexp.MustCompile(`^(?:(\d{1,2})d(\d{1,3})(?:\+(\d{1,5}))?|(\d{1,5}))(?:[x×](\d{1,4}))?$`)
}

// ParseAmount reads "3", "2d6", "1d4+1" or "4d6x10".
func ParseAmount(s string) (Amount, error) {
	m := amountPattern().FindStringSubmatch(s)
	if m == nil {
		return Amount{}, ErrAmount
	}
	a := Amount{Dice: 0, Faces: 0, Flat: 0, Times: 1}
	a.Dice, _ = strconv.Atoi(m[1])
	a.Faces, _ = strconv.Atoi(m[2])
	a.Flat, _ = strconv.Atoi(m[3] + m[4])
	if m[5] != "" {
		a.Times, _ = strconv.Atoi(m[5])
	}
	if (a.Dice > 0 && !dice.ValidFaces(a.Faces)) || a.Dice > dice.MaxDice || a.Times < 1 {
		return Amount{}, ErrAmount
	}
	return a, nil
}

// String writes the amount back the way ParseAmount reads it.
func (a Amount) String() string {
	s := strconv.Itoa(a.Flat)
	if a.Dice > 0 {
		s = strconv.Itoa(a.Dice) + "d" + strconv.Itoa(a.Faces)
		if a.Flat > 0 {
			s += "+" + strconv.Itoa(a.Flat)
		}
	}
	if a.Times > 1 {
		s += "x" + strconv.Itoa(a.Times)
	}
	return s
}

// Roll rolls the amount.
func (a Amount) Roll(src dice.Source) int {
	n := a.Flat
	for range a.Dice {
		n += dice.Face(src, a.Faces)
	}
	return n * a.Times
}

// Entry kinds of a Loot Table.
const (
	Item     = "item"
	Currency = "currency"
	Nested   = "table"
	Nothing  = "nothing"
)

// Entry is one weighted line of a Loot Table: items, coins of one kind, a roll on another table, or nothing.
type Entry struct {
	Weight int
	Kind   string
	Slug   string
	Coin   string
	Amount Amount
	Table  string
}

// Table is a Loot Table: how many times it is rolled and its entries.
type Table struct {
	Rolls   int
	Entries []Entry
}

// Drop is what a roll produced: Count of the item Slug, or of the Coin.
type Drop struct {
	Slug  string
	Coin  string
	Count int
}

// MaxDepth is how deep nested tables are followed; deeper rolls drop nothing.
const MaxDepth = 5

// Roll rolls a table and the tables it nests, adding up drops of the same thing in the order they first fell.
func Roll(src dice.Source, id string, tables map[string]Table) []Drop {
	var out []Drop
	roll(src, id, tables, 0, &out)
	return out
}

func roll(src dice.Source, id string, tables map[string]Table, depth int, out *[]Drop) {
	t, ok := tables[id]
	if !ok || depth >= MaxDepth {
		return
	}
	weights := make([]int, len(t.Entries))
	for i, e := range t.Entries {
		weights[i] = e.Weight
	}
	for range t.Rolls {
		i := encounters.Draw(src, weights)
		if i < 0 {
			return
		}
		e := t.Entries[i]
		switch e.Kind {
		case Item, Currency:
			add(out, Drop{Slug: e.Slug, Coin: e.Coin, Count: e.Amount.Roll(src)})
		case Nested:
			roll(src, e.Table, tables, depth+1, out)
		}
	}
}

func add(out *[]Drop, d Drop) {
	if d.Count <= 0 {
		return
	}
	for i, x := range *out {
		if x.Slug == d.Slug && x.Coin == d.Coin {
			(*out)[i].Count += d.Count
			return
		}
	}
	*out = append(*out, d)
}

// Coins are the coin kinds, cheapest first.
func Coins() []string {
	return []string{"cp", "sp", "ep", "gp", "pp"}
}

// CoinsPerPound is how many coins weigh a pound.
const CoinsPerPound = 50

// Capacity is how many pounds a creature carries: 15 times its Strength, scaled by its size.
func Capacity(strength int, size string) float64 {
	scale := map[string]float64{"tiny": 0.5, "large": 2, "huge": 4, "gargantuan": 8}[size]
	if scale == 0 {
		scale = 1
	}
	return float64(15*strength) * scale
}
