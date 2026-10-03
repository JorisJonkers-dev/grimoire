package domain

import (
	"maps"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// Map kinds: a local tactical map or a world map of locations and routes.
const (
	MapLocal = "local"
	MapWorld = "world"
)

// NodeID identifies a location on a world Map.
type NodeID uuid.UUID

// RouteID identifies a route between two locations.
type RouteID uuid.UUID

// WorldNode is a named location on a world Map. A Secret one is the DM's alone; LocalMap is the local
// Map that lies there, and LocalFound whether the party has found it.
type WorldNode struct {
	ID         NodeID
	Name       string
	At         hex.Coord
	Secret     bool
	LocalMap   *MapID
	LocalFound bool
}

// FoundMap is a Map the party finds or loses in play.
type FoundMap struct {
	Map MapID
	On  bool
}

// WorldRoute joins two locations; the party can travel it either way.
type WorldRoute struct {
	ID         RouteID
	From       NodeID
	To         NodeID
	DistanceMi int
}

// World is a world Map with its locations, routes, where the party stands and what the party has seen.
type World struct {
	Map     Map
	Nodes   []WorldNode
	Routes  []WorldRoute
	Party   *NodeID
	Reveals map[hex.Coord]bool
	// Legs are the Travel Legs the party made on this map this Session.
	Legs []TravelLeg
}

// Node finds a location by id.
func (w *World) Node(id NodeID) (WorldNode, bool) {
	i := slices.IndexFunc(w.Nodes, func(n WorldNode) bool { return n.ID == id })
	if i < 0 {
		return WorldNode{}, false
	}
	return w.Nodes[i], true
}

// Route finds a route by id.
func (w *World) Route(id RouteID) (WorldRoute, bool) {
	i := slices.IndexFunc(w.Routes, func(r WorldRoute) bool { return r.ID == id })
	if i < 0 {
		return WorldRoute{}, false
	}
	return w.Routes[i], true
}

// Clone copies the World so a change never touches the committed state.
func (w *World) Clone() *World {
	c := *w
	c.Nodes, c.Routes, c.Reveals, c.Legs = slices.Clone(w.Nodes), slices.Clone(w.Routes), maps.Clone(w.Reveals), slices.Clone(w.Legs)
	if w.Party != nil {
		p := *w.Party
		c.Party = &p
	}
	return &c
}

// TravelLeg is one journey of the party along a route. FromSecret and ToSecret say an end of it was a
// secret place when the party walked it: only the DM is told that end's name.
type TravelLeg struct {
	From       string
	To         string
	Pace       string
	DistanceMi int
	Minutes    int
	Days       int
	FromSecret bool
	ToSecret   bool
}

// World action kinds in the Action Log.
const (
	ActionWorldSet         = "world_set"
	ActionNodeAdded        = "node_added"
	ActionNodeRemoved      = "node_removed"
	ActionRouteAdded       = "route_added"
	ActionRouteRemoved     = "route_removed"
	ActionPartyPlaced      = "party_placed"
	ActionMapFound         = "map_found"
	ActionClockSet         = "clock_set"
	ActionMarchingOrderSet = "marching_order_set"
	ActionMapLost          = "map_lost"
	ActionTravelLeg        = "travel_leg"
)

// Random encounter action kinds in the Action Log.
const (
	ActionRestTaken         = "rest_taken"
	ActionCheckScheduled    = "check_scheduled"
	ActionEncounterChecked  = "encounter_checked"
	ActionEncounterResolved = "encounter_resolved"
)

// Standing is how a Faction of the Campaign regards the party, as a score, and the Characters whose
// Personal Standing with it is used instead.
type Standing struct {
	Faction  uuid.UUID
	Name     string
	Score    int
	Personal map[uuid.UUID]int
}
