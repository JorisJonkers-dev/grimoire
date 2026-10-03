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
	// CreationMethods are the ability score methods new Characters may use; StartingLevel is the level
	// they start at.
	CreationMethods []string
	StartingLevel   int
	// HoldLevelUps stops long rests unlocking the next level; the DM grants levels instead.
	HoldLevelUps bool
	// NoUndo plays the Campaign without undo: nothing is taken back, no Checkpoint is kept, no rewind.
	NoUndo bool
	// ExhaustionVariant is the exhaustion the Campaign plays with: srd-2024, gentle, grim or off.
	ExhaustionVariant string
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
	CreationMethods  []string
	StartingLevel    *int
	HoldLevelUps     *bool
	NoUndo           *bool
	// ExhaustionVariant picks the Campaign's exhaustion.
	ExhaustionVariant *string
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
	// Appearance and Backstory are written at creation and kept on the Character.
	Appearance string
	Backstory  string
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
	TempHP           int
	UpdatedAt        time.Time
	Portrait         *Image
	Token            *Image
	// Classes are its levels in each class, the starting class first; Picks and Spells what it chose on
	// the way; Increase its Ability Score Improvements. LevelUpReady means the next level is unlocked.
	Classes      []ClassLevel
	Picks        []Pick
	Spells       []LearnedSpell
	Increase     map[string]int
	LevelUpReady bool
	// CanPrepare means it may change its prepared spells: after a long rest or a new level.
	CanPrepare bool
	// HeroicInspiration is held or not; it is spent on a reroll or passed to an ally.
	HeroicInspiration bool
	// CarriedLb is what its Inventory weighs, coins included.
	CarriedLb float64
}

// ClassLevel is the levels a Character has in one class, and the subclass it chose there.
type ClassLevel struct {
	Class    string
	Subclass string
	Level    int
}

// Pick is one value chosen for a choice on reaching a level: "fighting-style" = "defense".
type Pick struct {
	Level  int
	Choice string
	Value  string
}

// LearnedSpell is a cantrip or spell a Character learned through a class, the level it learned it at,
// whether it is prepared, and whether it sits in a wizard's spellbook.
type LearnedSpell struct {
	Class     string
	Spell     string
	Level     int
	Prepared  bool
	Spellbook bool
}

// Clock is the time on a Campaign's Game Clock: its game day and the minutes after midnight.
type Clock struct {
	Day    int
	Minute int
}

// Purse is the coins in a Character's own container; Container is zero when it has none yet.
type Purse struct {
	Container uuid.UUID
	Coins     map[string]int
}

// LevelUp is a Character taking its next level: the hit points it gains, its classes afterwards, what it
// picked and learned, and its Ability Score Improvements afterwards.
type LevelUp struct {
	CampaignID CampaignID
	ID         CharacterID
	From       int
	Gain       int
	Classes    []ClassLevel
	Picks      []Pick
	Spells     []LearnedSpell
	Increase   map[string]int
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
	// HeroicInspiration shows in the party roster.
	HeroicInspiration bool
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

// ActionBars is how a player laid out a Character's actions in live play: up to two bars of ten tiles,
// a quick bar of four, and the tiles put away. A tile is named kind:name, such as attack:Longsword. Arranged is false until
// the player first saves a layout.
type ActionBars struct {
	Bars  [][]string `json:"bars"`
	Quick []string   `json:"quick"`
	// Stowed are the tiles the player took off the bars; any other tile the Character gains joins the end.
	Stowed   []string `json:"stowed"`
	Arranged bool     `json:"-"`
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

// Draft is a Character being made in the wizard: the step reached, the choices so far as the client
// keeps them, and the six scores the server rolled for it, if any.
type Draft struct {
	Step      int
	Build     []byte
	Rolled    []int
	UpdatedAt time.Time
}

// Snapshot is a Character's rebuildable choices at one moment: what a retrain proposes, or what a
// Revision kept.
type Snapshot struct {
	ID         uuid.UUID
	Species    string
	Background string
	Method     string
	Base       map[string]int
	Bonus      map[string]int
	Increase   map[string]int
	Skills     []string
	Picks      []Pick
	CreatedAt  time.Time
}

// Retrain statuses.
const (
	RetrainPending  = "pending"
	RetrainApproved = "approved"
	RetrainDeclined = "declined"
)

// Retrain is a player's request to rebuild a Campaign Character, which the DM approves or declines.
type Retrain struct {
	ID          uuid.UUID
	CharacterID CharacterID
	Proposed    Snapshot
	Status      string
	Reason      string
	RequestedBy string
	DecidedBy   string
	CreatedAt   time.Time
	DecidedAt   *time.Time
}

// CharacterRevision is a build an approved retrain replaced, kept as a Revision.
type CharacterRevision struct {
	No        int
	Author    string
	CreatedAt time.Time
	Build     Snapshot
	RetrainID *uuid.UUID
}
