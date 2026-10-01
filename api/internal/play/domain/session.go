package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/reactions"
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
	// Reactions are the Controller's settings, by kind of reaction prompt.
	Reactions map[string]reactions.Setting
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
	// Stealth and Perception are skill bonuses; Initiative the initiative bonus; SpeedFt the walking speed.
	Stealth    int
	Perception int
	Initiative int
	SpeedFt    int
	// UnarmedDC is the save a Grapple or Shove from this creature forces; AttacksPerAction is how many
	// attacks one Attack action holds (Extra Attack).
	UnarmedDC        int
	AttacksPerAction int
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
	// Light weapons open the off-hand attack; DamageMod is the ability modifier inside DamageBonus,
	// which an off-hand attack leaves out.
	Light     bool
	DamageMod int
	// Mastery is the weapon's mastery, when the creature has mastered it.
	Mastery string
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

// Kinds the DM's tools record: a whole encounter placed at once, and a hand-set hit point change.
const (
	ActionEncounterSpawned = "encounter_spawned"
	ActionHPAdjusted       = "hp_adjusted"
)

// LoggedAction is one Action of a Session as its log shows it: what it touched and whether it was undone.
type LoggedAction struct {
	Seq    int64
	Kind   string
	Actor  string
	Origin string
	Client string
	Label  string
	Undone bool
	At     time.Time
}

// Undoable reports whether the Action is one Grimoire can still revert.
func (a LoggedAction) Undoable() bool {
	switch a.Kind {
	case ActionTokenPlaced, ActionEncounterSpawned, ActionHexesRevealed, ActionHexesConcealed, ActionEffectApplied, ActionDamageDealt, ActionHPAdjusted:
		return !a.Undone
	}
	return false
}
