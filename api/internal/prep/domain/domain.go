// Package domain holds the prep model for random encounters: Encounter Pools, Encounter Tables and
// the Encounter Checks rolled against them.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// PoolID identifies an Encounter Pool.
type PoolID uuid.UUID

// TableID identifies an Encounter Table.
type TableID uuid.UUID

// PoolMember is a creature a Pool can field, how often, and how many at least and at most.
type PoolMember struct {
	Slug   string
	Weight int
	Min    int
	Max    int
}

// Pool is a weighted set of creatures for a band of party levels, filled to a difficulty's XP budget.
type Pool struct {
	ID         PoolID
	Name       string
	LevelMin   int
	LevelMax   int
	Difficulty string
	Members    []PoolMember
	UpdatedAt  time.Time
}

// Entry kinds of an Encounter Table.
const (
	EntryEncounter = "encounter"
	EntryPool      = "pool"
	EntryNothing   = "nothing"
)

// Visibility of an Encounter Table's checks: secret shows only the outcome, open shows the roll on the Table Display.
const (
	Secret = "secret"
	Open   = "open"
)

// EntryMonster is how many of one creature a prepared Encounter holds.
type EntryMonster struct {
	Slug  string
	Count int
}

// Entry is one weighted line of an Encounter Table: a prepared Encounter, a Pool draw, or Nothing.
type Entry struct {
	Weight   int
	Kind     string
	Label    string
	PoolID   *PoolID
	Monsters []EntryMonster
}

// Table is a Region's chance of an encounter and its weighted entries. A Table without a Region applies everywhere.
type Table struct {
	ID         TableID
	Name       string
	RegionID   *uuid.UUID
	ChancePct  int
	Visibility string
	Entries    []Entry
	UpdatedAt  time.Time
}

// Location is a place on a world map a Table can belong to.
type Location struct {
	ID      uuid.UUID
	Name    string
	MapName string
}

// Revisioned prep entity types.
const (
	EntityPool  = "encounter_pool"
	EntityTable = "encounter_table"
	EntityCheck = "encounter_check"
)

// CheckID identifies an Encounter Check.
type CheckID uuid.UUID

// What sets off an Encounter Check.
const (
	TriggerShortRest = "short_rest"
	TriggerLongRest  = "long_rest"
	TriggerTravelLeg = "travel_leg"
	TriggerDM        = "dm"
)

// How an Encounter Check draws: roll the chance, skip it and never draw Nothing, or take the DM's pick.
const (
	ModeNormal = "normal"
	ModeForce  = "force_encounter"
	ModePick   = "pick"
)

// Check statuses and outcomes.
const (
	CheckPending   = "pending"
	CheckResolved  = "resolved"
	OutcomeFight   = "encounter"
	OutcomeNothing = "nothing"
)

// When a scheduled check runs.
const (
	DueNextRest   = "next_rest"
	DueNextTravel = "next_travel"
)

// Check is one Encounter Check, seeded so its draw can be replayed, with what it produced.
type Check struct {
	ID         CheckID
	SessionID  *uuid.UUID
	TableID    *TableID
	TableName  string
	Trigger    string
	Mode       string
	Visibility string
	Seed       int64
	ChancePct  int
	// ChanceRoll is the percentile roll; 0 when the check skipped it.
	ChanceRoll int
	// RollID is the open check's Roll Request, rolled where the Table Display shows it.
	RollID     *uuid.UUID
	Status     string
	Outcome    string
	EntryLabel string
	Monsters   []EntryMonster
	CreatedAt  time.Time
}

// Scheduled is a check the DM asked for at the next rest or the next Travel Leg.
type Scheduled struct {
	ID      uuid.UUID
	TableID TableID
	Due     string
}

// Prep is everything an Encounter Check draws on: the Tables and Pools, each pooled creature's XP, the
// party's character levels, and the checks the DM scheduled.
type Prep struct {
	Tables    []Table
	Pools     []Pool
	XP        map[string]int
	Levels    []int
	Scheduled []Scheduled
}

// LootTableID identifies a Loot Table.
type LootTableID uuid.UUID

// LootEntry is one weighted line of a Loot Table: an Amount of an item, of coins of one kind, a roll
// on another Loot Table, or nothing.
type LootEntry struct {
	Weight int
	Kind   string
	Item   string
	Coin   string
	Amount string
	Table  *LootTableID
}

// LootTable is rolled Rolls times over its entries.
type LootTable struct {
	ID        LootTableID
	Name      string
	Rolls     int
	Entries   []LootEntry
	UpdatedAt time.Time
}

// EntityLoot is the revisioned entity type of Loot Tables.
const EntityLoot = "loot_table"
