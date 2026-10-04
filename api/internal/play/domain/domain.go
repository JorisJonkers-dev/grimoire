// Package domain holds the play context's rolls and Action Log.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// RollID identifies a Roll Request.
type RollID uuid.UUID

// Member is who acts in play, as the campaign context knows them.
type Member struct {
	ID      uuid.UUID
	Subject string
	Name    string
	DM      bool
}

// Modifier is a named flat bonus or penalty on a roll.
type Modifier struct {
	Label string
	Value int
}

// Die modes.
const (
	ModeAuto   = "auto"
	ModeManual = "manual"
)

// Die is one die of a Roll Request; Value is 0 until it is rolled or entered.
type Die struct {
	No    int
	Group int
	Faces int
	Value int
	Mode  string
	Kept  bool
	// KarmicDropped is set on a karmic d20: the other face the server rolled and let go.
	KarmicDropped *int
}

// Roll statuses.
const (
	StatusPending  = "pending"
	StatusResolved = "resolved"
)

// Roll is a Roll Request: exactly what to throw, why, and every modifier source.
type Roll struct {
	ID          RollID
	CampaignID  uuid.UUID
	Purpose     string
	Notation    string
	Labels      map[int]string
	Modifiers   []Modifier
	RequestedBy string
	Roller      Member
	Status      string
	Total       int
	Dice        []Die
	CreatedAt   time.Time
	ResolvedAt  time.Time
	// Choosing means every die is set and the roller, who holds Heroic Inspiration, keeps the roll or
	// spends it to reroll a die; Rerolled means it was spent on this roll.
	Choosing bool
	Rerolled bool
	// Asked means the roll was asked of its roller, by a fight or by a DM, rather than made by the
	// roller for themself: from the dice tray, or as a check taken at will outside a fight. Only an
	// asked roll can be karmic or count towards a run.
	Asked bool
}

// Action kinds in the Action Log.
const (
	ActionRollRequested = "roll_requested"
	ActionDieRolled     = "die_rolled"
	ActionDieEntered    = "die_entered"
	ActionRollResolved  = "roll_resolved"
)

// Action is one entry of the Action Log. Seed is set for dice the server rolled, so they replay exactly.
type Action struct {
	Seq       int64
	Kind      string
	Actor     string
	Origin    string
	Client    string
	Seed      *uint64
	RollID    *RollID
	DieNo     *int
	Value     int
	CreatedAt time.Time
}
