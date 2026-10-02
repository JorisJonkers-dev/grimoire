// Package domain holds the campaign context's entities and errors.
package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// Errors every campaign use case reports; adapters map each to one transport status.
var (
	ErrNotFound  = apperr.ErrNotFound
	ErrForbidden = apperr.ErrForbidden
	ErrConflict  = apperr.ErrConflict
	ErrInvalid   = apperr.ErrInvalid
	ErrLocked    = apperr.ErrLocked
)

type (
	// CampaignID identifies a Campaign.
	CampaignID uuid.UUID
	// MemberID identifies a Member.
	MemberID uuid.UUID
	// InviteID identifies an invite.
	InviteID uuid.UUID
	// CharacterID identifies a Campaign Character: a Character's progress in one Campaign.
	CharacterID uuid.UUID
	// OwnedID identifies a Character an Account owns (ADR-0010); its Campaign Characters point to it.
	OwnedID uuid.UUID
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
	// ReactionTimeoutS is how long a Reaction Prompt waits before it declines.
	ReactionTimeoutS int
	// HighGround turns on the optional rule: +2 to hit from higher ground.
	HighGround bool
	// RestSupplies turns on the optional rule: a Long Rest costs each resting Character a day of Rations.
	RestSupplies bool
	// InitiativeMode is "individual" or "side" (one roll per side); ShareInitiative gives identical
	// monsters one roll.
	InitiativeMode  string
	ShareInitiative bool
}

// SettingsChange is a change to a Campaign's settings; nil leaves a field alone.
type SettingsChange struct {
	Name             *string
	Ruleset          *string
	ReactionTimeoutS *int
	HighGround       *bool
	RestSupplies     *bool
	InitiativeMode   *string
	ShareInitiative  *bool
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

// Image is a stored picture, addressed by the hash of its content.
type Image struct {
	Key  string
	Type string
}

// ImageKind names which picture of a Character is meant.
type ImageKind string

// Image kinds.
const (
	Portrait  ImageKind = "portrait"
	TokenIcon ImageKind = "token"
)

// Character is a player character in a Campaign.
type Character struct {
	Build
	ID               CharacterID
	Owned            OwnedID
	CampaignID       CampaignID
	Owner            Member
	Ruleset          string
	Level            int
	BackgroundSkills []string
	HPMax            int
	HPCurrent        int
	UpdatedAt        time.Time
	Portrait         *Image
	Token            *Image
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
	TokenKey  string
}

// OwnedCharacter is a Character as its Account sees it: identity, build and Backstory, and its progress
// in each Campaign it plays in.
type OwnedCharacter struct {
	ID           OwnedID
	OwnerSubject string
	Name         string
	Ruleset      string
	Species      string
	Class        string
	Background   string
	Backstory    string
	HasPortrait  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Campaigns    []CampaignEntry
}

// CampaignEntry is one Campaign Character of an OwnedCharacter.
type CampaignEntry struct {
	CampaignID   CampaignID
	CampaignName string
	CharacterID  CharacterID
	Level        int
	HPCurrent    int
	HPMax        int
}
