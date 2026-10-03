package inventory_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
)

func TestSlotsTakeTheRightGear(t *testing.T) {
	t.Parallel()
	if got := len(inventory.Slots()); got != 14 {
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
		{inventory.RangedOff, "weapon", "dagger", true},
		{inventory.RangedOff, "armor", "shield", true},
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

func TestAttunementRequirements(t *testing.T) {
	t.Parallel()
	cases := []struct {
		detail  string
		classes []string
		caster  bool
		want    bool
	}{
		{"", []string{"fighter"}, false, true},
		{"Requires Attunement", []string{"fighter"}, false, true},
		{"Requires Attunement by a Spellcaster", []string{"fighter"}, false, false},
		{"Requires Attunement by a Spellcaster", []string{"fighter", "wizard"}, true, true},
		{"Requires Attunement by a Druid", []string{"wizard"}, true, false},
		{"Requires Attunement by a Bard, Cleric, or Druid", []string{"fighter", "cleric"}, true, true},
		{"Requires Attunement by a Sorcerer, Warlock, or Wizard", []string{"rogue"}, false, false},
	}
	for _, c := range cases {
		if got := inventory.CanAttune(c.detail, c.classes, c.caster); got != c.want {
			t.Errorf("%q for %v = %v", c.detail, c.classes, got)
		}
	}
	if inventory.MaxAttuned != 3 {
		t.Fatal("attunement slots")
	}
}

func TestUnknownItemsKeepTheirKindOnly(t *testing.T) {
	t.Parallel()
	for category, want := range map[string]string{
		"potion": "Unknown potion", "wondrous-item": "Unknown wondrous item", "ring": "Unknown ring", "": "Unknown item",
	} {
		if got := inventory.UnknownName(category); got != want {
			t.Errorf("%q = %q", category, got)
		}
	}
}

func TestChargesComeBackOnTheirSchedule(t *testing.T) {
	t.Parallel()
	// A dawn item waits for dawn: no rest gives its charges back, and each dawn does.
	wand := inventory.Charges{Max: 7, Dice: 1, Faces: 6, Bonus: 1, On: inventory.Dawn}
	for _, rest := range []inventory.Rest{inventory.ShortRest, inventory.LongRest} {
		if got := wand.Regain(rest, 2, 6); got != 2 {
			t.Errorf("a dawn item after a %s holds %d", rest, got)
		}
	}
	for _, c := range []struct{ current, rolled, want int }{{0, 4, 5}, {5, 6, 7}, {7, 1, 7}, {2, 0, 3}} {
		if got := wand.AtDawn(c.current, c.rolled); got != c.want {
			t.Errorf("at dawn %+v = %d", c, got)
		}
	}
	quick := inventory.Charges{Max: 3, Dice: 0, Faces: 0, Bonus: 3, On: inventory.ShortRestRecharge}
	if quick.Regain(inventory.ShortRest, 0, 0) != 3 || quick.Regain(inventory.LongRest, 1, 0) != 3 {
		t.Fatal("a short-rest item regains on any rest")
	}
	nightly := inventory.Charges{Max: 3, Dice: 0, Faces: 0, Bonus: 3, On: inventory.LongRestRecharge}
	if nightly.Regain(inventory.ShortRest, 0, 0) != 0 || nightly.Regain(inventory.LongRest, 0, 0) != 3 || nightly.Regain(inventory.LongRest, 1, 1) != 3 {
		t.Fatal("a long-rest item regains only on a long rest")
	}
	rod := inventory.Charges{Max: 10, Dice: 1, Faces: 4, Bonus: 1, On: inventory.LongRestRecharge}
	if got := rod.Regain(inventory.LongRest, 2, 3); got != 6 {
		t.Fatalf("a rod with 2 charges that rolls 3 and adds 1 holds %d", got)
	}
	// Dawn passes the items that recharge on a rest by.
	if quick.AtDawn(1, 0) != 1 || nightly.AtDawn(0, 2) != 0 {
		t.Fatal("a rest item regained at dawn")
	}
}

func TestWeaponSets(t *testing.T) {
	t.Parallel()
	if inventory.SetSlots(inventory.Melee) != [2]string{inventory.MainHand, inventory.OffHand} || inventory.SetSlots(inventory.RangedSet) != [2]string{inventory.Ranged, inventory.RangedOff} {
		t.Fatal("set slots")
	}
	if inventory.OtherSet(inventory.Melee) != inventory.RangedSet || inventory.OtherSet(inventory.RangedSet) != inventory.Melee {
		t.Fatal("other set")
	}
}
