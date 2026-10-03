package rolltable_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/rolltable"
)

func fumbles() rolltable.Design {
	return rolltable.Design{Dice: "1d6", Results: []rolltable.Result{
		{From: 1, To: 1, Text: "You fall flat on your face.", Effect: "prone"},
		{From: 2, To: 3, Text: "Your weapon slips from your grip."},
		{From: 5, To: 6, Text: "You find a coin in the dirt.", Item: "gold-piece", Quantity: 2},
	}}
}

// A Roll Table rolls plain dice of one kind, and its results cover ranges of what those dice can show,
// in order and without overlapping. A gap is allowed: nothing happens there.
func TestCheckingARollTable(t *testing.T) {
	t.Parallel()
	if err := rolltable.Check(fumbles()); err != nil {
		t.Fatalf("a sound table: %v", err)
	}
	change := func(f func(d *rolltable.Design)) rolltable.Design {
		d := fumbles()
		d.Results = slices.Clone(d.Results)
		f(&d)
		return d
	}
	hundred := rolltable.Design{Dice: "1d100"}
	for i := range 100 {
		hundred.Results = append(hundred.Results, rolltable.Result{From: i + 1, To: i + 1, Text: "x"})
	}
	for name, d := range map[string]rolltable.Design{
		"a hundred results":     hundred,
		"several dice":          {Dice: "2d6", Results: []rolltable.Result{{From: 2, To: 12, Text: "Anything."}}},
		"ten dice":              {Dice: "10d4", Results: []rolltable.Result{{From: 10, To: 40, Text: "Anything."}}},
		"upper case and spaces": {Dice: " 1D20 ", Results: []rolltable.Result{{From: 20, To: 20, Text: "Only on a 20."}}},
		"the longest words":     change(func(d *rolltable.Design) { d.Results[0].Text = strings.Repeat("a", 400) }),
		"the longest slugs": change(func(d *rolltable.Design) {
			d.Results[0].Effect, d.Results[2].Item = strings.Repeat("a", 80), strings.Repeat("b", 80)
		}),
		"an effect and an item at once": change(func(d *rolltable.Design) { d.Results[0].Item, d.Results[0].Quantity = "dagger", 1 }),
		"a hundred of an item":          change(func(d *rolltable.Design) { d.Results[2].Quantity = 100 }),
		"an item with no number":        change(func(d *rolltable.Design) { d.Results[2].Quantity = 0 }),
	} {
		if err := rolltable.Check(d); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	over := hundred
	over.Results = append(slices.Clone(hundred.Results), rolltable.Result{From: 100, To: 100, Text: "x"})
	for name, d := range map[string]rolltable.Design{
		"no dice":                       change(func(d *rolltable.Design) { d.Dice = "" }),
		"dice that are not dice":        change(func(d *rolltable.Design) { d.Dice = "a handful" }),
		"dice of two kinds":             change(func(d *rolltable.Design) { d.Dice = "1d6+1d4" }),
		"dice taken away":               change(func(d *rolltable.Design) { d.Dice = "-1d6" }),
		"dice that keep the highest":    change(func(d *rolltable.Design) { d.Dice = "2d6kh1" }),
		"eleven dice":                   change(func(d *rolltable.Design) { d.Dice = "11d6" }),
		"a die nobody owns":             change(func(d *rolltable.Design) { d.Dice = "1d7" }),
		"no results":                    change(func(d *rolltable.Design) { d.Results = nil }),
		"more than a hundred results":   over,
		"a result with no words":        change(func(d *rolltable.Design) { d.Results[1].Text = "  " }),
		"words too long":                change(func(d *rolltable.Design) { d.Results[1].Text = strings.Repeat("a", 401) }),
		"a range the wrong way round":   change(func(d *rolltable.Design) { d.Results[1].From, d.Results[1].To = 3, 2 }),
		"a range below the dice":        change(func(d *rolltable.Design) { d.Results[0].From = 0 }),
		"a range above the dice":        change(func(d *rolltable.Design) { d.Results[2].To = 7 }),
		"ranges that overlap":           change(func(d *rolltable.Design) { d.Results[1].From = 1 }),
		"ranges out of order":           change(func(d *rolltable.Design) { d.Results[0], d.Results[1] = d.Results[1], d.Results[0] }),
		"an effect slug too long":       change(func(d *rolltable.Design) { d.Results[0].Effect = strings.Repeat("a", 81) }),
		"an item slug too long":         change(func(d *rolltable.Design) { d.Results[2].Item = strings.Repeat("a", 81) }),
		"too many of an item":           change(func(d *rolltable.Design) { d.Results[2].Quantity = 101 }),
		"fewer than none of an item":    change(func(d *rolltable.Design) { d.Results[2].Quantity = -1 }),
		"a number of no item":           change(func(d *rolltable.Design) { d.Results[1].Quantity = 3 }),
		"two dice with a result of one": {Dice: "2d6", Results: []rolltable.Result{{From: 1, To: 12, Text: "Anything."}}},
	} {
		var bad rolltable.DesignError
		if err := rolltable.Check(d); !errors.As(err, &bad) || len(err.Error()) < 20 || !strings.HasSuffix(err.Error(), ".") {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Each refusal says what is wrong with the table.
	for want, d := range map[string]rolltable.Design{
		"dice of one kind":      change(func(d *rolltable.Design) { d.Dice = "1d6+1d4" }),
		"up to ten":             change(func(d *rolltable.Design) { d.Dice = "11d6" }),
		"between 1 and 100":     change(func(d *rolltable.Design) { d.Results = nil }),
		"needs words":           change(func(d *rolltable.Design) { d.Results[1].Text = "" }),
		"within 1 to 6":         change(func(d *rolltable.Design) { d.Results[2].To = 7 }),
		"do not overlap":        change(func(d *rolltable.Design) { d.Results[1].From = 1 }),
		"named by its slug":     change(func(d *rolltable.Design) { d.Results[0].Effect = strings.Repeat("a", 81) }),
		"up to 100 of its Item": change(func(d *rolltable.Design) { d.Results[2].Quantity = 101 }),
	} {
		if err := rolltable.Check(d); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v", want, err)
		}
	}
}

// A roll lands on the result whose range holds it, or on nothing in a gap or off the table.
func TestRollingOnATable(t *testing.T) {
	t.Parallel()
	d := fumbles()
	for total, want := range map[int]string{1: "You fall flat on your face.", 2: "Your weapon slips from your grip.", 3: "Your weapon slips from your grip.", 5: "You find a coin in the dirt.", 6: "You find a coin in the dirt."} {
		if r, ok := d.Roll(total); !ok || r.Text != want {
			t.Errorf("a %d = %+v %v", total, r, ok)
		}
	}
	for _, total := range []int{0, 4, 7, -1} {
		if r, ok := d.Roll(total); ok || r.Text != "" {
			t.Errorf("a %d = %+v %v", total, r, ok)
		}
	}
	if r, _ := d.Roll(1); r.Effect != "prone" || r.Item != "" || r.Gives() != 0 {
		t.Errorf("the first result = %+v gives %d", r, r.Gives())
	}
	if r, _ := d.Roll(6); r.Item != "gold-piece" || r.Gives() != 2 {
		t.Errorf("the last result = %+v gives %d", r, r.Gives())
	}
	// An item with no number given is one of it.
	if n := (rolltable.Result{Item: "dagger"}).Gives(); n != 1 {
		t.Errorf("an item with no number gives %d", n)
	}
}

// A table reads back as its dice and a line for each result.
func TestATableReadsBack(t *testing.T) {
	t.Parallel()
	got := rolltable.Lines(fumbles())
	want := []string{
		"Roll 1d6.",
		"1: You fall flat on your face. Applies prone.",
		"2-3: Your weapon slips from your grip.",
		"5-6: You find a coin in the dirt. Gives 2 × gold-piece.",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines = %q", got)
	}
	both := rolltable.Lines(rolltable.Design{Dice: "1D20", Results: []rolltable.Result{{From: 20, To: 20, Text: "A boon.", Effect: "blessed", Item: "potion-of-healing"}}})
	if !slices.Equal(both, []string{"Roll 1d20.", "20: A boon. Applies blessed. Gives 1 × potion-of-healing."}) {
		t.Fatalf("lines = %q", both)
	}
}
