package domain

import (
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
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
}

// ItemInfo is what an item is called and weighs.
type ItemInfo struct {
	Name     string
	WeightLb float64
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
