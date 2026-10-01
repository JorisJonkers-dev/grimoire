package domain

import (
	"maps"
	"time"

	"github.com/google/uuid"
)

// ContainerID identifies a Container.
type ContainerID uuid.UUID

// Container kinds: a Character's Inventory, the Party Stash, or a drop of loot.
const (
	ContainerCharacter = "character"
	ContainerStash     = "party_stash"
	ContainerDrop      = "loot_drop"
)

// Container holds items by slug and coins by kind.
type Container struct {
	ID          ContainerID
	Kind        string
	CharacterID *uuid.UUID
	Label       string
	Items       map[string]int
	Coins       map[string]int
	CreatedAt   time.Time
}

// Clone copies the Container so a change never touches the committed state.
func (c Container) Clone() Container {
	c.Items, c.Coins = maps.Clone(c.Items), maps.Clone(c.Coins)
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
	From      ContainerID
	To        ContainerID
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
