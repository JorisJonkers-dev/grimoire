package domain

import (
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
)

// ContainerID identifies a Container.
type ContainerID uuid.UUID

// Container kinds: a Character's Inventory, the Party Stash, a drop of loot, or a bag inside another Container.
const (
	ContainerCharacter = "character"
	ContainerStash     = "party_stash"
	ContainerDrop      = "loot_drop"
	ContainerBag       = "bag"
)

// InstanceID identifies an Item Instance.
type InstanceID uuid.UUID

// Instance is one Item Instance: a base or homebrew item with its own name, Charges, identified and
// attuned state and equipped slot. Plain stackable gear keeps a Quantity instead.
type Instance struct {
	ID         InstanceID
	Slug       string
	CustomName string
	Quantity   int
	Charges    *int
	Identified bool
	Attuned    bool
	Slot       string
}

// Plain reports whether an Instance is a plain stack: nothing singles it out, so it merges with others
// of its item.
func (in Instance) Plain() bool {
	return in.CustomName == "" && in.Charges == nil && in.Slot == "" && !in.Attuned && in.Identified
}

// Container holds plain stacks by slug, the Item Instances singled out, and coins by kind.
type Container struct {
	ID          ContainerID
	Kind        string
	CharacterID *uuid.UUID
	// ParentID is the Container a bag sits in.
	ParentID  *ContainerID
	Label     string
	Items     map[string]int
	Instances []Instance
	Coins     map[string]int
	CreatedAt time.Time
}

// Clone copies the Container so a change never touches the committed state.
func (c Container) Clone() Container {
	c.Items, c.Coins, c.Instances = maps.Clone(c.Items), maps.Clone(c.Coins), slices.Clone(c.Instances)
	return c
}

// Bearer is a Character who carries an Inventory: whose it is and how strong they are.
type Bearer struct {
	CharacterID uuid.UUID
	Name        string
	Owner       uuid.UUID
	Strength    int
	// Classes decide which items it may attune to.
	Classes []string
	// WeaponSet is the set of weapons in hand: melee or ranged.
	WeaponSet string
}

// ItemInfo is what an item is called and weighs.
type ItemInfo struct {
	Name     string
	WeightLb float64
	// Category is the compendium's kind of item: weapon, armor, potion, ring, wondrous-item and so on.
	Category string
	// RequiresAttunement and AttunementDetail ("Requires Attunement by a Druid") say who may attune it.
	RequiresAttunement bool
	AttunementDetail   string
	// MaxCharges is how many charges it holds, 0 for none; it regains RegainDice d RegainFaces plus
	// RegainBonus on RechargeOn: dawn, long_rest or short_rest.
	MaxCharges  int
	RegainDice  int
	RegainFaces int
	RegainBonus int
	RechargeOn  string
}

// Inventory is every Container of a Campaign, who carries them, and what their items are.
type Inventory struct {
	Containers []Container
	Bearers    []Bearer
	Items      map[string]ItemInfo
}

// Move is a number of one item, or of one kind of coin, going from one Container to another.
type Move struct {
	From ContainerID
	To   ContainerID
	// Instance is the Item Instance moved whole, if it is one rather than part of a plain stack.
	Instance  *InstanceID
	Item      string
	Coin      string
	Count     int
	FromLabel string
	ToLabel   string
	// Left and Now are what the two Containers hold of it afterwards.
	Left int
	Now  int
}

// Inventory action kinds in the Action Log.
const (
	ActionLootDropped = "loot_dropped"
	ActionItemMoved   = "item_moved"
	ActionCoinsMoved  = "coins_moved"
)

// Held is what a Character wears and holds in a weapon set: its armor, whether a shield is in the set's
// off hand, and the set's weapons.
func (inv Inventory) Held(mine Container, set string) (string, bool, []string) {
	hands := inventory.SetSlots(set)
	armor, shield, weapons := "", false, []string{}
	for _, in := range mine.Instances {
		switch {
		case in.Slot == inventory.Body:
			armor = in.Slug
		case in.Slot == hands[1] && in.Slug == "shield":
			shield = true
		case slices.Contains(hands[:], in.Slot) && inv.Items[in.Slug].Category == "weapon":
			weapons = append(weapons, in.Slug)
		}
	}
	return armor, shield, weapons
}

// Carrier is the Character's own Container and its Bearer, if it carries one.
func (inv Inventory) Carrier(character uuid.UUID) (Container, Bearer, bool) {
	b := slices.IndexFunc(inv.Bearers, func(x Bearer) bool { return x.CharacterID == character })
	c := slices.IndexFunc(inv.Containers, func(x Container) bool {
		return x.Kind == ContainerCharacter && x.CharacterID != nil && *x.CharacterID == character
	})
	if b < 0 || c < 0 {
		return Container{}, Bearer{}, false
	}
	return inv.Containers[c], inv.Bearers[b], true
}
