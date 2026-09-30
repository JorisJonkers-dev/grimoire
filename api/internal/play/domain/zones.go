package domain

import (
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// ZoneID identifies an Encounter Zone.
type ZoneID uuid.UUID

// Encounter Zone statuses: waiting for the party, waiting on Perception rolls, or done.
const (
	ZoneArmed    = "armed"
	ZoneSpotting = "spotting"
	ZoneSprung   = "sprung"
)

// Zone is an Encounter Zone: hidden creatures within RadiusHexes of At spring on the party. A DMOnly
// zone never springs by itself, and a Held one waits until the DM lets it.
type Zone struct {
	ID          ZoneID
	Name        string
	At          hex.Coord
	RadiusHexes int
	DMOnly      bool
	Held        bool
	Status      string
	// DC is the hidden creatures' Stealth DC, set when the zone springs.
	DC int
	// Creatures are the hidden creatures the zone held when it sprang.
	Creatures []TokenID
	Checks    []ZoneCheck
}

// Covers reports whether a hex lies within the zone.
func (z Zone) Covers(c hex.Coord) bool {
	return hex.Distance(z.At, c) <= z.RadiusHexes
}

// Decided reports whether every party member has noticed the ambush or not.
func (z Zone) Decided() bool {
	return !slices.ContainsFunc(z.Checks, func(c ZoneCheck) bool { return c.Noticed == nil })
}

// ZoneCheck is whether one party member noticed a sprung zone: by passive Perception, or by the
// Perception roll RollID once it resolves.
type ZoneCheck struct {
	Token   TokenID
	RollID  *RollID
	Noticed *bool
}

// Encounter Zone action kinds in the Action Log.
const (
	ActionZoneAdded        = "zone_added"
	ActionZoneRemoved      = "zone_removed"
	ActionZoneHeld         = "zone_held"
	ActionZoneSprung       = "zone_sprung"
	ActionPerceptionRolled = "perception_rolled"
)
