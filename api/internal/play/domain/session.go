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
	// WorldMapID is the world Map the party travels this Session.
	WorldMapID *MapID
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
	// Tactics is the DM's override of how the creature picks Suggested Actions; "auto" follows Intelligence.
	Tactics string
	// CanShield offers the Shield reaction when the token is hit.
	CanShield bool
}

// Stats is a token's fighting statblock, copied from a monster or a Character when it is placed.
type Stats struct {
	Source  string
	AC      int
	HP      int
	HPMax   int
	Attacks []Attack
	// Intelligence drives Tactics; 0 when the statblock has none.
	Intelligence int
	// Shield is set for statblocks that can cast the Shield spell.
	Shield bool
	// Saves are saving throw bonuses by ability.
	Saves map[string]int
	// SpellDC is the save DC of the token's spells; 0 when it casts none.
	SpellDC int
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
	ActionTacticsSet     = "tactics_set"
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
