package inventory_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
)

func TestSlotsTakeTheRightGear(t *testing.T) {
	t.Parallel()
	if got := len(inventory.Slots()); got != 13 {
		t.Fatalf("slots = %d", got)
	}
	fits := []struct {
		slot, category, slug string
		want                 bool
	}{
		{inventory.MainHand, "weapon", "longsword", true},
		{inventory.MainHand, "armor", "chain-mail", false},
		{inventory.OffHand, "weapon", "dagger", true},
		{inventory.OffHand, "armor", "shield", true},
		{inventory.OffHand, "armor", "chain-mail", false},
		{inventory.Ranged, "weapon", "longbow", true},
		{inventory.Ranged, "ammunition", "arrows-20", false},
		{inventory.Body, "armor", "chain-mail", true},
		{inventory.Body, "armor", "shield", false},
		{inventory.Head, "wondrous-item", "helm-of-brilliance", true},
		{inventory.Cloak, "wondrous-item", "cloak-of-protection", true},
		{inventory.Hands, "wondrous-item", "gauntlets-of-ogre-power", true},
		{inventory.Feet, "wondrous-item", "boots-of-speed", true},
		{inventory.Amulet, "wondrous-item", "amulet-of-health", true},
		{inventory.Amulet, "potion", "potion-of-healing", false},
		{inventory.Ring1, "ring", "ring-of-protection", true},
		{inventory.Ring2, "ring", "ring-of-protection", true},
		{inventory.Ring1, "wondrous-item", "amulet-of-health", false},
		{inventory.Ammunition, "ammunition", "arrows-20", true},
		{inventory.Instrument, "tools", "musical-instrument-lute", true},
		{inventory.Instrument, "tools", "thieves-tools", false},
		{"pocket", "weapon", "dagger", false},
	}
	for _, c := range fits {
		if got := inventory.Fits(c.slot, c.category, c.slug); got != c.want {
			t.Errorf("%s takes %s %s = %v, want %v", c.slot, c.category, c.slug, got, c.want)
		}
	}
}

func TestCarryingCapacityAndSpeed(t *testing.T) {
	t.Parallel()
	if got := inventory.Capacity(15); got != 225 {
		t.Fatalf("capacity = %v", got)
	}
	for _, c := range []struct {
		weight float64
		want   inventory.Load
		speed  int
	}{
		{0, inventory.Unburdened, 30}, {225, inventory.Unburdened, 30}, {225.5, inventory.Encumbered, 5}, {450, inventory.Encumbered, 5}, {451, inventory.Immobile, 0},
	} {
		load := inventory.LoadOf(c.weight, 225)
		if load != c.want || inventory.Speed(30, load) != c.speed {
			t.Errorf("%v lb = %v %d", c.weight, load, inventory.Speed(30, load))
		}
	}
	if got := inventory.CoinWeight(map[string]int{"gp": 120, "sp": 30}); got != 3 {
		t.Fatalf("coin weight = %v", got)
	}
}

func TestHealingPotions(t *testing.T) {
	t.Parallel()
	for slug, want := range map[string]inventory.Potion{
		"potion-of-healing": {Dice: 2, Faces: 4, Bonus: 2}, "potion-of-greater-healing": {Dice: 4, Faces: 4, Bonus: 4},
		"potion-of-superior-healing": {Dice: 8, Faces: 4, Bonus: 8}, "potion-of-supreme-healing": {Dice: 10, Faces: 4, Bonus: 20},
	} {
		if got, ok := inventory.Healing(slug); !ok || got != want {
			t.Errorf("%s = %+v %v", slug, got, ok)
		}
	}
	if _, ok := inventory.Healing("potion-of-heroism"); ok {
		t.Fatal("heroism heals")
	}
}
