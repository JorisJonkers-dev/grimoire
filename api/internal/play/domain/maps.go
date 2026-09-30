package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// MapID identifies a local Map.
type MapID uuid.UUID

// Ambient light levels of a Map.
const (
	AmbientBright = "bright"
	AmbientDim    = "dim"
	AmbientDark   = "dark"
)

// Map is an uploaded picture calibrated to the hex grid.
type Map struct {
	ID         MapID
	CampaignID uuid.UUID
	Name       string
	ImageKey   string
	ImageType  string
	Width      int
	Height     int
	HexSize    float64
	OriginX    float64
	OriginY    float64
	Ambient    string
	UpdatedAt  time.Time
}

// Layout is the Map's pixel geometry.
func (m Map) Layout() hex.Layout {
	return hex.Layout{Size: m.HexSize, Origin: hex.Point{X: m.OriginX, Y: m.OriginY}}
}

// LightID identifies a light on a Map.
type LightID uuid.UUID

// MapLight is a light the DM placed.
type MapLight struct {
	ID       LightID
	At       hex.Coord
	BrightFt int
	DimFt    int
}

// MapState is a Map with everything vision and fog need: walls, lights and what the party has seen.
type MapState struct {
	Map     Map
	Walls   map[hex.Coord]bool
	Lights  []MapLight
	Reveals map[hex.Coord]bool
}

// Map action kinds in the Action Log.
const (
	ActionMapSet         = "map_set"
	ActionHexesRevealed  = "hexes_revealed"
	ActionHexesConcealed = "hexes_concealed"
	ActionWallsSet       = "walls_set"
	ActionWallsCleared   = "walls_cleared"
	ActionLightPlaced    = "light_placed"
	ActionLightRemoved   = "light_removed"
	ActionAmbientSet     = "ambient_set"
)
