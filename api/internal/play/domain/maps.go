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

// How a Map's grid is drawn over its picture. A local Map is always hexes of 5 feet.
const (
	GridHexes   = "hexes"
	GridSquares = "squares"
	GridOff     = "off"
)

// LocalHexFt is how far across one hex of a local Map is.
const LocalHexFt = 5

// Map is an uploaded picture calibrated to the hex grid. GridStrength is how solid the grid is drawn,
// 0 to 100; ScaleMiles is how many miles one cell of a world Map covers.
type Map struct {
	ID           MapID
	CampaignID   uuid.UUID
	Name         string
	Kind         string
	ImageKey     string
	ImageType    string
	Width        int
	Height       int
	HexSize      float64
	OriginX      float64
	OriginY      float64
	Ambient      string
	Grid         string
	GridStrength int
	ScaleMiles   float64
	// Found says the party has obtained this Map in play.
	Found     bool
	UpdatedAt time.Time
}

// CellSpan is the distance one cell covers: feet on a local Map, miles on a world Map.
func (m Map) CellSpan() float64 {
	if m.Kind == MapWorld {
		return m.ScaleMiles
	}
	return LocalHexFt
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
	// A trap is Armed until disarmed or sprung; DetectDC is what a passive Perception must reach to find
	// it, DisarmDC what thieves' tools must beat, and TriggerFt how close a creature comes to set it off.
	Armed     bool
	DetectDC  int
	DisarmDC  int
	TriggerFt int
	// A Locked object opens only by its Key, thieves' tools or force against LockDC, Knock, or breaking.
	Locked bool
	LockDC int
	Key    string
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
	ActionObjectPlaced   = "object_placed"
	ActionObjectRemoved  = "object_removed"
	ActionObjectToggled  = "object_toggled"
	ActionObjectDamaged  = "object_damaged"
	ActionObjectFound    = "object_found"
	ActionObjectUnlocked = "object_unlocked"
	ActionTrapDisarmed   = "trap_disarmed"
	ActionTrapSprung     = "trap_sprung"
	// ActionJumped is a creature leaping; ActionThrown one throwing a creature or an object.
	ActionJumped = "jumped"
	ActionThrown = "thrown"
	// Sneaking action kinds.
	ActionSneakStarted  = "sneak_started"
	ActionSneakEnded    = "sneak_ended"
	ActionStealthRolled = "stealth_rolled"
	ActionPartyNoticed  = "party_noticed"
	// Exploration turn action kinds.
	ActionExplorationStarted = "exploration_started"
	ActionExplorationTurn    = "exploration_turn"
	ActionExplorationEnded   = "exploration_ended"
)

// Sneak is the party moving quietly with a group Stealth check: each member's roll and, once rolled,
// its total.
type Sneak struct {
	Rolls []SneakRoll
}

// SneakRoll is one member's Stealth roll for a Sneak.
type SneakRoll struct {
	Token  TokenID
	RollID RollID
	Total  *int
}

// Exploration is the party moving in turns outside a fight: the order, whose turn it is, and how far
// that member has moved this turn.
type Exploration struct {
	Order   []TokenID
	Turn    int
	MovedFt int
}
