package domain

import (
	"time"

	"github.com/google/uuid"
)

// QuestID identifies a Quest; LoreID a Lore entry.
type (
	QuestID = uuid.UUID
	LoreID  = uuid.UUID
)

// Statuses of a Quest. A hidden one is the DM's alone until it is given to the party.
const (
	QuestHidden    = "hidden"
	QuestActive    = "active"
	QuestCompleted = "completed"
	QuestFailed    = "failed"
)

// QuestStep is one step of a Quest, done or still to do.
type QuestStep struct {
	Text string
	Done bool
}

// Quest is a goal the party pursues, with steps and a status.
type Quest struct {
	ID         QuestID
	CampaignID CampaignID
	Name       string
	Summary    string
	Status     string
	Steps      []QuestStep
	UpdatedAt  time.Time
}

// Lore is world knowledge of the Campaign. It is the DM's alone until the party unlocks it; ItemSlug
// names the book or letter whose reading unlocks it, when there is one.
type Lore struct {
	ID         LoreID
	CampaignID CampaignID
	Title      string
	Body       string
	ItemSlug   string
	UnlockedAt *time.Time
	UpdatedAt  time.Time
}
