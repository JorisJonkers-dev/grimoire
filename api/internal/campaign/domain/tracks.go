package domain

import (
	"time"

	"github.com/google/uuid"
)

// TrackID identifies a Track.
type TrackID = uuid.UUID

// Track scopes: a score kept for each Character, or one for the whole party.
const (
	TrackPerCharacter = "character"
	TrackParty        = "party"
)

// TrackThreshold is a score of a Track at which something happens, on the way up when it is rising and
// on the way down otherwise: an Effect, a roll on a Roll Table, or only its label.
type TrackThreshold struct {
	At        int
	Rising    bool
	Label     string
	Effect    string
	RollTable *uuid.UUID
}

// Track is a Campaign-specific score such as sanity or renown, kept within bounds.
type Track struct {
	ID         TrackID
	CampaignID CampaignID
	Name       string
	Scope      string
	Min        int
	Max        int
	Start      int
	Thresholds []TrackThreshold
	CreatedAt  time.Time
}

// TrackValue is where a Track stands for one Character, or for the party when Character is nil.
type TrackValue struct {
	Track     TrackID
	Character *CharacterID
	Value     int
}

// TrackCharacter is a Character a Track can be kept for.
type TrackCharacter struct {
	ID    CharacterID
	Name  string
	Owner MemberID
}
