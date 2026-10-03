package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
)

// Downtime activities.
const (
	DowntimeCraft    = "craft"
	DowntimeWork     = "work"
	DowntimeTrain    = "train"
	DowntimeResearch = "research"
)

// DowntimeCharacter is a Character with its downtime days: the ones left, and the ones lived through
// in the downtime the DM last gave the whole party.
type DowntimeCharacter struct {
	ID    uuid.UUID
	Name  string
	Owner uuid.UUID
	Days  int
	Spent int
}

// CampaignRecipe is a Recipe a Campaign crafts from.
type CampaignRecipe struct {
	ID     uuid.UUID
	Recipe downtime.Recipe
}

// DowntimeEntry is something a Character did with its downtime.
type DowntimeEntry struct {
	ID        uuid.UUID
	Campaign  uuid.UUID
	Character uuid.UUID
	Name      string
	Activity  string
	Detail    string
	Days      int
	At        time.Time
}

// Downtime is a Campaign's downtime: its Game Clock and how far this downtime has moved it, its
// Characters' days, its Recipes and what was done.
type Downtime struct {
	Clock      clock.Time
	Advanced   int
	Characters []DowntimeCharacter
	Recipes    []CampaignRecipe
	Log        []DowntimeEntry
}
