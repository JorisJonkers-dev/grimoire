package domain

import (
	"time"

	"github.com/google/uuid"
)

// HookID identifies a Rule Variant a DM authored.
type HookID = uuid.UUID

// RuleHook is a Rule Variant a DM authors for a Campaign: at a hook point it applies an Effect to
// whoever it happened to, or has them roll on a Roll Table.
type RuleHook struct {
	ID         HookID
	CampaignID CampaignID
	Name       string
	Hook       string
	RollTable  *uuid.UUID
	Effect     string
	CreatedAt  time.Time
}

// RollTableRef names a Roll Table of the Library a Campaign sees.
type RollTableRef struct {
	ID   uuid.UUID
	Name string
}
