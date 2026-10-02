package loot_test

import (
	"maps"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
)

func TestNeedBeatsGreedThenTheRollThenTheEarlierClaim(t *testing.T) {
	ranked := loot.Rank([]loot.Claim{
		{Claimant: "brom", Choice: loot.Greed, Roll: 20, Order: 0},
		{Claimant: "cara", Choice: loot.Need, Roll: 7, Order: 3},
		{Claimant: "aria", Choice: loot.Need, Roll: 7, Order: 2},
		{Claimant: "dain", Choice: loot.Need, Roll: 12, Order: 4},
		{Claimant: "eve", Choice: loot.Need, Roll: 7, Order: 2},
	})
	var got []string
	for _, c := range ranked {
		got = append(got, c.Claimant)
	}
	if want := []string{"dain", "aria", "eve", "cara"}; !equal(got, want) {
		t.Fatalf("ranked = %v, want %v", got, want)
	}
	greed := loot.Rank([]loot.Claim{{Claimant: "brom", Choice: loot.Greed, Roll: 3}, {Claimant: "aria", Choice: loot.Greed, Roll: 9}})
	if len(greed) != 2 || greed[0].Claimant != "aria" {
		t.Fatalf("greed alone = %+v", greed)
	}
	if loot.Rank(nil) != nil {
		t.Fatal("no claims rank nothing")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSharingDealsUnitsInRankOrder(t *testing.T) {
	ranked := []loot.Claim{{Claimant: "dain"}, {Claimant: "aria"}}
	for n, want := range map[int]map[string]int{
		1: {"dain": 1},
		2: {"dain": 1, "aria": 1},
		5: {"dain": 3, "aria": 2},
	} {
		if got := loot.Share(n, ranked); !maps.Equal(got, want) {
			t.Errorf("share %d = %v, want %v", n, got, want)
		}
	}
	if got := loot.Share(3, nil); len(got) != 0 {
		t.Fatalf("no claimants share nothing = %v", got)
	}
}

func TestCoinsSplitExactlyDownToCopper(t *testing.T) {
	for _, c := range []struct {
		coins      map[string]int
		k          int
		each, left map[string]int
	}{
		{map[string]int{"gp": 9}, 3, map[string]int{"gp": 3}, map[string]int{}},
		{map[string]int{"gp": 10}, 3, map[string]int{"gp": 3, "sp": 3, "cp": 3}, map[string]int{"cp": 1}},
		{map[string]int{"pp": 1}, 4, map[string]int{"gp": 2, "sp": 5}, map[string]int{}},
		{map[string]int{"ep": 1}, 2, map[string]int{"sp": 2, "cp": 5}, map[string]int{}},
		{map[string]int{"ep": 3, "sp": 1}, 2, map[string]int{"ep": 1, "sp": 3}, map[string]int{}},
		{map[string]int{"cp": 5}, 2, map[string]int{"cp": 2}, map[string]int{"cp": 1}},
		{map[string]int{"gp": 3, "sp": 1}, 1, map[string]int{"gp": 3, "sp": 1}, map[string]int{}},
		{map[string]int{"gp": 4}, 0, map[string]int{}, map[string]int{"gp": 4}},
		{map[string]int{}, 3, map[string]int{}, map[string]int{}},
	} {
		each, left := loot.Split(c.coins, c.k)
		if !maps.Equal(each, c.each) || !maps.Equal(left, c.left) {
			t.Errorf("split %v by %d = %v each, %v left; want %v, %v", c.coins, c.k, each, left, c.each, c.left)
		}
	}
}
