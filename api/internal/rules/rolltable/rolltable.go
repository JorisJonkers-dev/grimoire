// Package rolltable holds Roll Tables: dice, ranges of what they can show, and for each range a result
// that may apply an Effect or give an Item.
package rolltable

import (
	"fmt"
	"slices"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// DesignError is why a table cannot be used, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Limits on a table.
const (
	MaxResults  = 100
	MaxDice     = 10
	MaxText     = 400
	MaxSlug     = 80
	MaxQuantity = 100
)

// Result is what a range of the dice gives: words, and maybe an Effect on whoever rolled or an Item.
type Result struct {
	From     int    `json:"from"`
	To       int    `json:"to"`
	Text     string `json:"text"`
	Effect   string `json:"effect,omitempty"`
	Item     string `json:"item,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
}

// Gives is how many of its Item a result gives: none without one, one when no number is given.
func (r Result) Gives() int {
	if r.Item == "" {
		return 0
	}
	return max(r.Quantity, 1)
}

// Design is a Roll Table as its author builds it.
type Design struct {
	Dice    string   `json:"dice"`
	Results []Result `json:"results"`
}

func faces() []int { return []int{4, 6, 8, 10, 12, 20, 100} }

// rolled reads a table's dice: up to ten of one kind, added up.
func rolled(notation string) (dice.Group, error) {
	spec, err := dice.Parse(strings.TrimSpace(notation))
	if err != nil || len(spec.Groups) != 1 {
		return dice.Group{}, DesignError("A table rolls dice of one kind, such as 1d20 or 2d6.")
	}
	g := spec.Groups[0]
	if g.Sign != 1 || g.Keep != dice.KeepAll || g.Count > MaxDice || !slices.Contains(faces(), g.Faces) {
		return dice.Group{}, DesignError("A table rolls up to ten d4, d6, d8, d10, d12, d20 or d100, added up.")
	}
	return g, nil
}

func checkResult(r Result, lo, hi, after int) error {
	if strings.TrimSpace(r.Text) == "" || len([]rune(r.Text)) > MaxText {
		return DesignError("Each result needs words, up to 400 characters.")
	}
	if r.From > r.To || r.From < lo || r.To > hi {
		return DesignError(fmt.Sprintf("Each range runs from its lower number to its higher, within %d to %d.", lo, hi))
	}
	if r.From <= after {
		return DesignError("The ranges go in order and do not overlap.")
	}
	if len(r.Effect) > MaxSlug || len(r.Item) > MaxSlug {
		return DesignError("An Effect or Item is named by its slug, up to 80 characters.")
	}
	if r.Quantity < 0 || r.Quantity > MaxQuantity || (r.Item == "" && r.Quantity != 0) {
		return DesignError("A result gives up to 100 of its Item.")
	}
	return nil
}

// Check reports why a design cannot be used, or nil.
func Check(d Design) error {
	g, err := rolled(d.Dice)
	if err != nil {
		return err
	}
	if len(d.Results) == 0 || len(d.Results) > MaxResults {
		return DesignError("A table has between 1 and 100 results.")
	}
	lo, hi := g.Count, g.Count*g.Faces
	after := lo - 1
	for _, r := range d.Results {
		if err := checkResult(r, lo, hi, after); err != nil {
			return err
		}
		after = r.To
	}
	return nil
}

// Roll finds the result a total lands on; a total in a gap or off the table lands on nothing.
func (d Design) Roll(total int) (found Result, ok bool) {
	for _, r := range d.Results {
		if total >= r.From && total <= r.To {
			return r, true
		}
	}
	return found, false
}

// Notation is the table's dice as the dice roller reads them.
func (d Design) Notation() string {
	return strings.ToLower(strings.TrimSpace(d.Dice))
}

// Lines reads a table back: its dice, then a line for each result.
func Lines(d Design) []string {
	out := []string{"Roll " + d.Notation() + "."}
	for _, r := range d.Results {
		span := fmt.Sprintf("%d-%d", r.From, r.To)
		if r.From == r.To {
			span = fmt.Sprintf("%d", r.From)
		}
		line := span + ": " + r.Text
		if r.Effect != "" {
			line += " Applies " + r.Effect + "."
		}
		if r.Item != "" {
			line += fmt.Sprintf(" Gives %d × %s.", r.Gives(), r.Item)
		}
		out = append(out, line)
	}
	return out
}
