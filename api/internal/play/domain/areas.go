package domain

import (
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// Surface is terrain on a hex; RoundsLeft of 0 lasts until it changes.
type Surface struct {
	Kind       surface.Kind
	RoundsLeft int
}

// AreaCast is an area spell waiting on its damage roll and its targets' saving throws.
type AreaCast struct {
	ID         uuid.UUID
	Caster     TokenID
	Spell      string
	DC         int
	DamageRoll *RollID
	Hexes      []hex.Coord
	Targets    []AreaTarget
}

// AreaTarget is a creature in an area; SaveRoll is nil for spells without a save.
type AreaTarget struct {
	Token    TokenID
	SaveRoll *RollID
}

// Area and terrain action kinds in the Action Log.
const (
	ActionAreaCast     = "area_cast"
	ActionAreaResolved = "area_resolved"
	ActionSurfacesSet  = "surfaces_set"
	ActionElevationSet = "elevation_set"
)
