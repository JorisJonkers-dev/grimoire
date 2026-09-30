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
}

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
