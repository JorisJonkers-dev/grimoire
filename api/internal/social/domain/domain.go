// Package domain holds Friends (and, later, Conversations): who is connected to whom, keyed on
// Accounts.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors the social service returns.
var (
	ErrNoAccount = errors.New("social: no Account")
	ErrNotFound  = errors.New("social: not found")
	ErrConflict  = errors.New("social: already so")
	ErrInvalid   = errors.New("social: invalid")
)

// AccountID identifies an Account.
type AccountID = uuid.UUID

// Person is another Account as social pages show it.
type Person struct {
	ID       AccountID
	Username string
	Nickname string
}

// Friend is a Person in a mutual, accepted friendship.
type Friend struct {
	Person
	Since time.Time
}

// Request is a Friend request waiting for an answer, with the other side of it.
type Request struct {
	ID     uuid.UUID
	Person Person
	At     time.Time
}

// Friends is everything the Friends page shows.
type Friends struct {
	Friends  []Friend
	Incoming []Request
	Outgoing []Request
	Blocked  []Request
}

// Mention kinds: a Campaign Character, or a Location on a Campaign's world map.
const (
	MentionCharacter = "character"
	MentionLocation  = "location"
)

// Mention is game content a Message points at.
type Mention struct {
	Kind       string
	CampaignID uuid.UUID
	TargetID   uuid.UUID
}

// Placed is a stored Mention and the Message it belongs to.
type Placed struct {
	Message uuid.UUID
	Mention
}

// Resolved is a Mention as one reader sees it: its name and where it opens, when they may open it.
type Resolved struct {
	Mention
	Label string
	MapID uuid.UUID
	Open  bool
}

// Conversation is a Conversation as its members' list shows it.
type Conversation struct {
	ID        uuid.UUID
	Title     string
	Members   []Person
	UpdatedAt time.Time
	Unread    int
	LastBody  string
}

// Message is one message in a Conversation, with its Mentions as the reader sees them.
type Message struct {
	ID       uuid.UUID
	Author   Person
	Body     string
	At       time.Time
	Mentions []Resolved
}

// Mentionable is game content the caller may mention.
type Mentionable struct {
	Kind         string
	ID           uuid.UUID
	Name         string
	CampaignID   uuid.UUID
	CampaignName string
}
