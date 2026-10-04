package guides_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/guides"
)

// The loot guide has four tiers of play; each opens one more rarity, and none offers an artifact.
func TestLootTiers(t *testing.T) {
	t.Parallel()
	want := []guides.Tier{
		{No: 1, From: 1, To: 4, Rarities: []string{"common", "uncommon"}},
		{No: 2, From: 5, To: 10, Rarities: []string{"common", "uncommon", "rare"}},
		{No: 3, From: 11, To: 16, Rarities: []string{"common", "uncommon", "rare", "very-rare"}},
		{No: 4, From: 17, To: 20, Rarities: []string{"common", "uncommon", "rare", "very-rare", "legendary"}},
	}
	got := guides.Tiers()
	if len(got) != len(want) {
		t.Fatalf("tiers = %+v", got)
	}
	for i := range want {
		if got[i].No != want[i].No || got[i].From != want[i].From || got[i].To != want[i].To || !slices.Equal(got[i].Rarities, want[i].Rarities) {
			t.Errorf("tier %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}
	// Each call gives its own copy: changing one does not change the guide.
	got[0].Rarities[0] = "artifact"
	if guides.Tiers()[0].Rarities[0] != "common" {
		t.Errorf("the tiers were changed from outside")
	}
	for rarity, tier := range map[string]int{"common": 1, "uncommon": 1, "rare": 2, "very-rare": 3, "legendary": 4, "artifact": 0, "": 0, "mythic": 0} {
		if got := guides.FirstTier(rarity); got != tier {
			t.Errorf("%q first suits tier %d, want %d", rarity, got, tier)
		}
	}
}

// A Challenge Rating reads as the books write it.
func TestChallenge(t *testing.T) {
	t.Parallel()
	for cr, want := range map[float64]string{0: "0", 0.125: "1/8", 0.25: "1/4", 0.5: "1/2", 1: "1", 2: "2", 10: "10", 30: "30", 0.75: "0.75"} {
		if got := guides.Challenge(cr); got != want {
			t.Errorf("CR %v reads %q, want %q", cr, got, want)
		}
	}
}

// A hit deals its dice at their average, plus its bonus, plus any extra dice; an attack with no dice
// deals its flat damage, and dice that cannot be read count for nothing.
func TestAverageDamage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a    guides.Attack
		want float64
	}{
		{guides.Attack{Dice: "1d6", Bonus: 2}, 5.5},
		{guides.Attack{Dice: "2d6", Bonus: 3}, 10},
		{guides.Attack{Dice: "1d8"}, 4.5},
		{guides.Attack{Dice: "2d10", Bonus: 5, ExtraDice: "2d6"}, 23},
		{guides.Attack{Bonus: 1}, 1},
		{guides.Attack{Dice: "1d4", Bonus: -1}, 1.5},
		{guides.Attack{Dice: "lots", Bonus: 4, ExtraDice: "more"}, 4},
		{guides.Attack{}, 0},
		{guides.Attack{Dice: "1d1", Bonus: -5}, 0},
	} {
		if got := guides.Average(c.a); got != c.want {
			t.Errorf("%+v deals %v on average, want %v", c.a, got, c.want)
		}
	}
}

// The attacks of one Challenge Rating are summed up by the lowest, the middle and the highest bonus to
// hit and the middle damage of a hit.
func TestSummarise(t *testing.T) {
	t.Parallel()
	hit := func(toHit int, dice string, bonus int) guides.Attack {
		return guides.Attack{ToHit: toHit, Dice: dice, Bonus: bonus}
	}
	for name, c := range map[string]struct {
		attacks []guides.Attack
		want    guides.Band
	}{
		"none":  {nil, guides.Band{}},
		"one":   {[]guides.Attack{hit(4, "1d6", 2)}, guides.Band{Attacks: 1, ToHitLow: 4, ToHit: 4, ToHitHigh: 4, Damage: 5}},
		"three": {[]guides.Attack{hit(7, "2d6", 4), hit(3, "1d4", 1), hit(5, "1d8", 3)}, guides.Band{Attacks: 3, ToHitLow: 3, ToHit: 5, ToHitHigh: 7, Damage: 7}},
		// With an even number the middle is the lower of the two in the middle.
		"four": {[]guides.Attack{hit(9, "3d6", 0), hit(2, "1d4", 0), hit(6, "1d10", 0), hit(4, "1d6", 0)}, guides.Band{Attacks: 4, ToHitLow: 2, ToHit: 4, ToHitHigh: 9, Damage: 3}},
		"two":  {[]guides.Attack{hit(8, "1d12", 5), hit(6, "1d6", 1)}, guides.Band{Attacks: 2, ToHitLow: 6, ToHit: 6, ToHitHigh: 8, Damage: 4}},
	} {
		given := slices.Clone(c.attacks)
		if got := guides.Summarise(c.attacks); got != c.want {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
		}
		if !slices.Equal(given, c.attacks) {
			t.Errorf("%s: the attacks were put in another order", name)
		}
	}
}
