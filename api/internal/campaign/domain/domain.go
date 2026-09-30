// Package domain holds the campaign context's entities and errors.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors every campaign use case reports; adapters map each to one transport status.
var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
	ErrInvalid   = errors.New("invalid")
	ErrLocked    = errors.New("locked")
)

type (
	// CampaignID identifies a Campaign.
	CampaignID uuid.UUID
	// MemberID identifies a Member.
	MemberID uuid.UUID
	// InviteID identifies an invite.
	InviteID uuid.UUID
	// CharacterID identifies a Character.
	CharacterID uuid.UUID
)

// ListCursor is where the next page of a caller's Campaigns starts.
type ListCursor struct {
	CreatedAt time.Time
	ID        CampaignID
}

// Role is what a Member may do in a Campaign.
type Role string

// Roles.
const (
	RoleDM     Role = "dm"
	RolePlayer Role = "player"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	return r == RoleDM || r == RolePlayer
}

// Campaign is one ongoing game.
type Campaign struct {
	ID        CampaignID
	Name      string
	Ruleset   string
	CreatedAt time.Time
}

// Member is an account's participation in a Campaign.
type Member struct {
	ID          MemberID
	CampaignID  CampaignID
	Subject     string
	DisplayName string
	Role        Role
	JoinedAt    time.Time
}

// Summary is a Campaign as it appears in the caller's list.
type Summary struct {
	Campaign
	MyRole      Role
	MemberCount int
}

// Detail is a Campaign's home: the campaign, the caller's membership and every member.
type Detail struct {
	Campaign
	Me      Member
	Members []Member
}

// Invite is an open invitation link; only its hash is stored.
type Invite struct {
	ID            InviteID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	CreatedByName string
}

// NewInvite is an invite together with its token, which is shown once.
type NewInvite struct {
	Invite
	Token string
}

// InvitePreview is what someone holding an invite link sees before joining.
type InvitePreview struct {
	CampaignID   CampaignID
	CampaignName string
	InvitedBy    string
}

// Build is what a player chooses for a first-level Character.
type Build struct {
	Name       string
	Species    string
	Class      string
	Background string
	Method     string
	Base       map[string]int
	Bonus      map[string]int
	Skills     []string
	Armor      string
	Shield     bool
	Weapons    []string
}

// Character is a player character in a Campaign.
type Character struct {
	Build
	ID               CharacterID
	CampaignID       CampaignID
	Owner            Member
	Ruleset          string
	Level            int
	BackgroundSkills []string
	HPMax            int
	HPCurrent        int
	UpdatedAt        time.Time
}

// CharacterSummary is a Character as it appears in the party list.
type CharacterSummary struct {
	ID        CharacterID
	Name      string
	OwnerName string
	Mine      bool
	Species   string
	Class     string
	Level     int
	HPCurrent int
	HPMax     int
}
