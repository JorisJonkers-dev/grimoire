package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionID identifies a Session.
type SessionID uuid.UUID

// Session statuses.
const (
	SessionLive  = "live"
	SessionEnded = "ended"
)

// Session is one evening of play.
type Session struct {
	ID         SessionID
	CampaignID uuid.UUID
	Number     int
	Status     string
	Seq        int64
	GridRadius int
	StartedAt  time.Time
	EndedAt    time.Time
	MapID      *MapID
}

// TokenID identifies a Token.
type TokenID uuid.UUID

// Token kinds.
const (
	TokenParty  = "party"
	TokenEnemy  = "enemy"
	TokenNPC    = "npc"
	TokenObject = "object"
)

// Token is a marker on the Session's map.
type Token struct {
	ID     TokenID
	Label  string
	Kind   string
	Q      int
	R      int
	Hidden bool
	// DarkvisionFt lets a party token see in darkness.
	DarkvisionFt int
	// Controller is the member who may walk the token besides the DM.
	Controller *uuid.UUID
	// Stats is the statblock a token fights with; nil for markers and objects.
	Stats *Stats
}

// Stats is a token's fighting statblock, copied from a monster or a Character when it is placed.
type Stats struct {
	Source  string
	AC      int
	HP      int
	HPMax   int
	Attacks []Attack
}

// Attack is one attack on a token's hotbar. Damage is dice notation, empty for flat damage.
type Attack struct {
	Name        string
	ToHit       int
	ReachFt     int
	RangeFt     int
	LongRangeFt int
	Damage      string
	DamageBonus int
	DamageType  string
}

// Attack action kinds in the Action Log.
const (
	ActionAttackDeclared = "attack_declared"
	ActionAttackMissed   = "attack_missed"
	ActionAttackHit      = "attack_hit"
	ActionDamageDealt    = "damage_dealt"
	ActionDamageUndone   = "damage_undone"
)

// Token action kinds in the Action Log.
const (
	ActionSessionStarted = "session_started"
	ActionSessionEnded   = "session_ended"
	ActionTokenPlaced    = "token_placed"
	ActionTokenMoved     = "token_moved"
	ActionTokenHidden    = "token_hidden"
	ActionTokenRevealed  = "token_revealed"
	ActionTokenRemoved   = "token_removed"
	ActionTokenWalked    = "token_walked"
)
