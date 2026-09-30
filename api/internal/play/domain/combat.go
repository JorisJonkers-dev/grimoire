package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
)

// CombatID identifies a Combat.
type CombatID uuid.UUID

// CombatantID identifies a Combatant.
type CombatantID uuid.UUID

// Combat statuses: everyone rolls initiative, then turns run; an ended Combat is history.
const (
	CombatRolling = "rolling"
	CombatActive  = "active"
	CombatEnded   = "ended"
)

// Combat is a running fight inside a live Session.
type Combat struct {
	ID     CombatID
	Status string
	Round  int
	// Turn is the initiative count now acting; every Combatant on it acts at the same time.
	Turn       int
	Combatants []Combatant
	StartedAt  time.Time
	EndedAt    time.Time
}

// Combatant is a Token taking part in a Combat.
type Combatant struct {
	ID              CombatantID
	TokenID         TokenID
	RollID          RollID
	InitiativeBonus int
	SpeedFt         int
	// Initiative is nil until the initiative Roll Request resolves.
	Initiative *int
	// Done is set once the Combatant ends its turn this round.
	Done    bool
	Economy combat.Economy
}

// Totals lists every rolled initiative.
func (c *Combat) Totals() []int {
	var out []int
	for _, x := range c.Combatants {
		if x.Initiative != nil {
			out = append(out, *x.Initiative)
		}
	}
	return out
}

// Acting reports whether a Combatant's turn is now.
func (c *Combat) Acting(x Combatant) bool {
	return c.Status == CombatActive && x.Initiative != nil && *x.Initiative == c.Turn && !x.Done
}

// Combat action kinds in the Action Log.
const (
	ActionCombatStarted    = "combat_started"
	ActionInitiativeRolled = "initiative_rolled"
	ActionTurnEnded        = "turn_ended"
	ActionResourceSpent    = "resource_spent"
	ActionCombatEnded      = "combat_ended"
)
