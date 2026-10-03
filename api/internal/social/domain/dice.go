package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DiceSetID identifies a Dice Set.
type DiceSetID = uuid.UUID

// Who a Dice Set is shared with.
const (
	SharingPrivate  = "private"
	SharingFriends  = "friends"
	SharingEveryone = "everyone"
)

// Where a Dice Set stands with the Admins. Only a set shared with everyone that carries an uploaded
// picture is reviewed.
const (
	ReviewNone     = "none"
	ReviewPending  = "pending"
	ReviewApproved = "approved"
	ReviewRejected = "rejected"
)

// DiePlacement is where a set's uploaded picture sits on one die's unwrapped faces: its centre as a
// fraction of the sheet, its scale, and its turn in degrees.
type DiePlacement struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Scale    float64 `json:"scale"`
	Rotation float64 `json:"rotation"`
}

// DieLook is how one type of die looks: a preset pattern in two colours, with the set's picture on it
// when it is placed. Numbers are drawn on their own layer, so any picture stays readable.
type DieLook struct {
	Pattern string        `json:"pattern"`
	Body    string        `json:"body"`
	Numbers string        `json:"numbers"`
	Image   *DiePlacement `json:"image,omitempty"`
}

// DiceDesign is a Dice Set's looks by die type: d4, d6, d8, d10, d12, d20 and d100. A die type it
// leaves out rolls plain.
type DiceDesign struct {
	Dice map[string]DieLook `json:"dice"`
}

// DieTypes are the dice a Dice Set can dress.
func DieTypes() []string {
	return []string{"d4", "d6", "d8", "d10", "d12", "d20", "d100"}
}

// DicePatterns are the preset patterns.
func DicePatterns() []string {
	return []string{"plain", "marble", "speckled", "stripes"}
}

// Picture is an uploaded picture in the asset store.
type Picture struct {
	Key  string
	Type string
}

// Digest is the SHA-256 of the picture's content, which names it: two pictures share one only when
// they are the same picture. A key that is no content hash has none.
func (p Picture) Digest() string {
	sum, _, _ := strings.Cut(strings.TrimPrefix(p.Key, "sha256/"), ".")
	if len(sum) != 64 || sum == p.Key {
		return ""
	}
	return sum
}

// DiceSet is the look an Account gives its dice. A copy was taken from a set shared with its owner: it
// cannot be edited or shared on, and it stays when the sharing stops. MadeBy is the Username of whoever
// designed it.
type DiceSet struct {
	ID         DiceSetID
	Owner      AccountID
	Name       string
	Design     DiceDesign
	Image      *Picture
	Sharing    string
	Review     string
	CopiedFrom *DiceSetID
	MadeBy     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Copy reports whether the set is a copy of someone else's.
func (d DiceSet) Copy() bool {
	return d.CopiedFrom != nil
}

// Public reports whether everyone may see the set: shared with everyone, and either without a picture
// or with one an Admin approved.
func (d DiceSet) Public() bool {
	return d.Sharing == SharingEveryone && (d.Image == nil || d.Review == ReviewApproved)
}

// Cleared reports whether the set carries a picture an Admin approved. Such a picture may show on any
// screen; a copy of the set carries the approval with it.
func (d DiceSet) Cleared() bool {
	return d.Image != nil && d.Review == ReviewApproved
}
