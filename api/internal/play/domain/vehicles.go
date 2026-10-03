package domain

import (
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
)

// VehiclePart is a component of a vehicle, with the hit points it has left.
type VehiclePart struct {
	ID     uuid.UUID
	Name   string
	HP     int
	HPMax  int
	Drives bool
}

// VehiclePost is a crew station of a vehicle, with how many crew are posted there.
type VehiclePost struct {
	ID     uuid.UUID
	Name   string
	Crew   int
	Posted int
}

// Vehicle is a vehicle or ship of a Campaign as it stands.
type Vehicle struct {
	ID          uuid.UUID
	Name        string
	Kind        string
	Hull        int
	HullMax     int
	Threshold   int
	MilesPerDay int
	Parts       []VehiclePart
	Posts       []VehiclePost
}

// State is what the vehicle's speed goes by: its hull, its working drives and whether it is short of crew.
func (v Vehicle) State() vehicles.State {
	at := vehicles.State{Hull: v.Hull, Drives: 0, Working: 0, ShortHanded: false}
	for _, p := range v.Parts {
		if p.Drives {
			at.Drives++
		}
		if p.Drives && p.HP > 0 {
			at.Working++
		}
	}
	for _, p := range v.Posts {
		at.ShortHanded = at.ShortHanded || p.Posted < p.Crew
	}
	return at
}

// Speed is the miles the vehicle covers in a day as it stands.
func (v Vehicle) Speed() int {
	return vehicles.Speed(v.MilesPerDay, v.State())
}
