package domain

import (
	"time"

	"github.com/google/uuid"
)

// FactionID identifies a Faction of a Campaign; ChangeID a Standing Change.
type (
	FactionID = uuid.UUID
	ChangeID  = uuid.UUID
)

// Statuses of a Standing Change: it waits for the DM, who confirms or dismisses it.
const (
	ChangePending   = "pending"
	ChangeConfirmed = "confirmed"
	ChangeDismissed = "dismissed"
)

// Faction is an organisation of the Campaign's world. Score is how it regards the party, kept as a
// number only the DM sees; everyone else is shown the tier it reads as.
type Faction struct {
	ID         FactionID
	CampaignID CampaignID
	Name       string
	Archetype  string
	Goals      string
	Territory  string
	Notes      string
	Score      int
	UpdatedAt  time.Time
}

// PersonalStanding is one Character's own Standing with a Faction, used instead of the party's.
type PersonalStanding struct {
	FactionID   FactionID
	CharacterID CharacterID
	Character   string
	Owner       MemberID
	Score       int
}

// StandingChange is a move in Standing with a reason. It is pending until the DM confirms or dismisses
// it; Character is nil for the party. ShareReason says whether Players are shown the reason.
type StandingChange struct {
	ID          ChangeID
	FactionID   FactionID
	Character   *CharacterID
	Delta       int
	Reason      string
	ShareReason bool
	Status      string
	Origin      string
	Client      string
	ProposedBy  string
	CreatedAt   time.Time
	DecidedAt   *time.Time
}
