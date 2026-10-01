package loot_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
)

// script answers IntN from a fixed list.
type script struct {
	t      *testing.T
	values []int
}

func (s *script) IntN(n int) int {
	if len(s.values) == 0 {
		s.t.Fatalf("no value left for IntN(%d)", n)
	}
	v := s.values[0]
	if v >= n {
		s.t.Fatalf("IntN(%d) cannot answer %d", n, v)
	}
	s.values = s.values[1:]
	return v
}

func TestAmountsReadRollAndWriteBack(t *testing.T) {
	for text, want := range map[string]loot.Amount{
		"3":      {Flat: 3, Times: 1},
		"2d6":    {Dice: 2, Faces: 6, Times: 1},
		"1d4+1":  {Dice: 1, Faces: 4, Flat: 1, Times: 1},
		"4d6x10": {Dice: 4, Faces: 6, Times: 10},
		"5×100":  {Flat: 5, Times: 100},
		"20d100": {Dice: 20, Faces: 100, Times: 1},
	} {
		got, err := loot.ParseAmount(text)
		if err != nil || got != want {
			t.Errorf("%q = %+v %v", text, got, err)
		}
	}
	for _, bad := range []string{"", "d6", "2d7", "21d6", "1d4+", "+3", "3x0", "x2", "2d6-1", "abc"} {
		if _, err := loot.ParseAmount(bad); !errors.Is(err, loot.ErrAmount) {
			t.Errorf("%q = %v", bad, err)
		}
	}
	for _, text := range []string{"3", "2d6", "1d4+1", "4d6x10", "5x100", "0"} {
		a, _ := loot.ParseAmount(text)
		if a.String() != text {
			t.Errorf("%q writes back as %q", text, a.String())
		}
	}
	if got := (loot.Amount{Dice: 2, Faces: 6, Flat: 1, Times: 10}).Roll(&script{t: t, values: []int{2, 4}}); got != (3+5+1)*10 {
		t.Errorf("roll = %d", got)
	}
}

func TestRollWalksNestedTablesAndAddsUpDrops(t *testing.T) {
	gp, _ := loot.ParseAmount("2d6")
	one, _ := loot.ParseAmount("1")
	none, _ := loot.ParseAmount("0")
	tables := map[string]loot.Table{
		"hoard": {Rolls: 3, Entries: []loot.Entry{
			{Weight: 1, Kind: loot.Currency, Coin: "gp", Amount: gp},
			{Weight: 1, Kind: loot.Nested, Table: "gems"},
			{Weight: 1, Kind: loot.Nothing},
			{Weight: 1, Kind: loot.Item, Slug: "rope", Amount: none},
		}},
		"gems":  {Rolls: 2, Entries: []loot.Entry{{Weight: 3, Kind: loot.Item, Slug: "pearl", Amount: one}, {Weight: 1, Kind: loot.Nested, Table: "gone"}}},
		"loop":  {Rolls: 1, Entries: []loot.Entry{{Weight: 1, Kind: loot.Nested, Table: "loop"}, {Weight: 1, Kind: loot.Item, Slug: "ring", Amount: one}}},
		"empty": {Rolls: 2, Entries: []loot.Entry{}},
	}
	// hoard: gp (2d6 = 3+4), gems (pearl, pearl), gp (1+1), then nothing would be a fourth roll.
	src := &script{t: t, values: []int{0, 2, 3, 1, 0, 0, 0, 0, 0}}
	got := loot.Roll(src, "hoard", tables)
	if want := []loot.Drop{{Coin: "gp", Count: 9}, {Slug: "pearl", Count: 2}}; !slices.Equal(got, want) {
		t.Fatalf("hoard = %+v, want %+v", got, want)
	}
	if got := loot.Roll(&script{t: t, values: []int{2, 3}}, "hoard", map[string]loot.Table{"hoard": {Rolls: 2, Entries: tables["hoard"].Entries}}); got != nil {
		t.Fatalf("nothing and zero rope = %+v", got)
	}
	deep := loot.Roll(&script{t: t, values: []int{0, 0, 0, 0, 0}}, "loop", tables)
	if deep != nil {
		t.Fatalf("a table nesting itself stops at %d levels = %+v", loot.MaxDepth, deep)
	}
	if got := loot.Roll(&script{t: t, values: []int{1}}, "loop", tables); !slices.Equal(got, []loot.Drop{{Slug: "ring", Count: 1}}) {
		t.Fatalf("loop's ring = %+v", got)
	}
	if loot.Roll(&script{t: t}, "empty", tables) != nil || loot.Roll(&script{t: t}, "missing", tables) != nil {
		t.Fatal("empty and missing tables drop nothing")
	}
}

func TestCarryingCapacityAndCoins(t *testing.T) {
	for _, c := range []struct {
		str  int
		size string
		want float64
	}{{10, "medium", 150}, {10, "small", 150}, {8, "tiny", 60}, {18, "large", 540}, {20, "huge", 1200}, {20, "gargantuan", 2400}} {
		if got := loot.Capacity(c.str, c.size); got != c.want {
			t.Errorf("Capacity(%d, %s) = %v, want %v", c.str, c.size, got, c.want)
		}
	}
	if !slices.Equal(loot.Coins(), []string{"cp", "sp", "ep", "gp", "pp"}) || loot.CoinsPerPound != 50 {
		t.Error("coins")
	}
}
