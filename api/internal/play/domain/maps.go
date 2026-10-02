package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// MapID identifies a Map.
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
	Kind       string
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
	// Elevation is each raised or sunken hex's height in feet.
	Elevation map[hex.Coord]int
	// Objects are the Map Objects on the Map.
	Objects map[ObjectID]MapObject
}

// ObjectID identifies a Map Object.
type ObjectID = uuid.UUID

// MapObject is an interactable thing on a local Map, with its own Armor Class and hit points. A Secret
// object is unknown to the party until found; Effect fires on whoever uses it, or on every creature
// within RadiusFt, when it is used or broken; Links are the objects a lever works.
type MapObject struct {
	ID       ObjectID
	Kind     string
	Name     string
	At       hex.Coord
	AC       int
	HP       int
	HPMax    int
	Open     bool
	Broken   bool
	Secret   bool
	Effect   string
	RadiusFt int
	Links    []ObjectID
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
	// Map Object action kinds.
	ActionObjectPlaced  = "object_placed"
	ActionObjectRemoved = "object_removed"
	ActionObjectToggled = "object_toggled"
	ActionObjectDamaged = "object_damaged"
	ActionObjectFound   = "object_found"
)
