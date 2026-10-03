package domain

import (
	"time"

	"github.com/google/uuid"
)

// CompanionID identifies a Companion.
type CompanionID = uuid.UUID

// Companion kinds: an ally who travels with the party of their own will, or one who is paid to.
const (
	KindCompanion = "companion"
	KindHireling  = "hireling"
)

// Companion is an ally who travels with the party: a creature with a name of its own. Controller is
// the Player who runs it, nil when the DM does. SharesXP gives it a share of every XP Award, which
// nobody else gets. HP is what it had when it last left the map, nil while it has never been hurt.
type Companion struct {
	ID          CompanionID
	CampaignID  CampaignID
	Name        string
	Kind        string
	MonsterSlug string
	Controller  *MemberID
	SharesXP    bool
	HP          *int
	Notes       string
	UpdatedAt   time.Time
}
