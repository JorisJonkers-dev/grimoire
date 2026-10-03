// Package vehicles holds the rules of vehicles and ships: what one is made of, the damage its hull and
// components take, how fast it goes with what is left of it, and how long a Travel Leg aboard takes.
package vehicles

import "unicode/utf8"

// DesignError is why a vehicle cannot be kept, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Kinds of vehicle, by what it travels over.
const (
	Land  = "land"
	Water = "water"
	Air   = "air"
)

// Limits on a vehicle.
const (
	MaxName        = 80
	MaxHP          = 10000
	MaxThreshold   = 100
	MaxMilesPerDay = 1000
	MaxParts       = 20
	MaxCrew        = 200
)

// Component is a part of a vehicle with hit points of its own; one that Drives moves the vehicle.
type Component struct {
	Name   string
	HPMax  int
	Drives bool
}

// Station is a crew station and how many crew it takes to man it.
type Station struct {
	Name string
	Crew int
}

// Design is a vehicle as it is built: its hull, the damage threshold under which a blow does nothing,
// and the miles it covers in a day's travel when whole and fully crewed.
type Design struct {
	Name        string
	Kind        string
	HullMax     int
	Threshold   int
	MilesPerDay int
	Components  []Component
	Stations    []Station
}

func named(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= MaxName
}

// Check reports why a vehicle cannot be kept.
func (d Design) Check() error {
	if !named(d.Name) {
		return DesignError("Give the vehicle a name of up to 80 characters.")
	}
	if d.Kind != Land && d.Kind != Water && d.Kind != Air {
		return DesignError("A vehicle goes by land, water or air.")
	}
	if d.HullMax < 1 || d.HullMax > MaxHP {
		return DesignError("A hull has 1 to 10000 hit points.")
	}
	if d.Threshold < 0 || d.Threshold > MaxThreshold {
		return DesignError("A damage threshold runs from 0 to 100.")
	}
	if d.MilesPerDay < 1 || d.MilesPerDay > MaxMilesPerDay {
		return DesignError("A vehicle covers 1 to 1000 miles a day.")
	}
	if err := checkComponents(d.Components); err != nil {
		return err
	}
	return checkStations(d.Stations)
}

func checkComponents(list []Component) error {
	if len(list) > MaxParts {
		return DesignError("A vehicle has up to 20 components.")
	}
	seen := map[string]bool{}
	for _, c := range list {
		if !named(c.Name) {
			return DesignError("Give each component a name of up to 80 characters.")
		}
		if seen[c.Name] {
			return DesignError("Name each component once.")
		}
		if c.HPMax < 1 || c.HPMax > MaxHP {
			return DesignError("A component has 1 to 10000 hit points.")
		}
		seen[c.Name] = true
	}
	return nil
}

func checkStations(list []Station) error {
	if len(list) > MaxParts {
		return DesignError("A vehicle has up to 20 crew stations.")
	}
	seen := map[string]bool{}
	for _, s := range list {
		if !named(s.Name) {
			return DesignError("Give each crew station a name of up to 80 characters.")
		}
		if seen[s.Name] {
			return DesignError("Name each crew station once.")
		}
		if s.Crew < 0 || s.Crew > MaxCrew {
			return DesignError("A crew station takes 0 to 200 crew.")
		}
		seen[s.Name] = true
	}
	return nil
}

// Hit is the hit points a hull or a component is left with after a blow: one under the vehicle's
// damage threshold does nothing, and any other lands in full.
func Hit(hp, threshold, amount int) int {
	if amount < threshold {
		return hp
	}
	return max(0, hp-amount)
}

// Mend is the hit points a repair leaves, never more than the part can have.
func Mend(hp, most, amount int) int {
	return min(most, hp+amount)
}

// State is how a vehicle stands: its hull, how many of its driving components still work, and
// whether any crew station is short of crew.
type State struct {
	Hull        int
	Drives      int
	Working     int
	ShortHanded bool
}

// Speed is the miles a vehicle covers in a day as it stands: none as a wreck, the share its working
// drives give of what it was built for, and half of that with a short crew.
func Speed(milesPerDay int, at State) int {
	if at.Hull <= 0 {
		return 0
	}
	speed := milesPerDay
	if at.Drives > 0 {
		speed = milesPerDay * at.Working / at.Drives
	}
	if at.ShortHanded {
		speed /= 2
	}
	return speed
}

// hoursByLand is a day's travel for a vehicle drawn or driven over land.
const hoursByLand = 8

// hoursUnderWay is a day's travel for a ship or an airship, whose crew keeps watches.
const hoursUnderWay = 24

// HoursPerDay is how long a vehicle travels in a day: a ship or an airship round the clock, a land
// vehicle as long as the beasts or the walkers with it do.
func HoursPerDay(kind string) int {
	if kind == Land {
		return hoursByLand
	}
	return hoursUnderWay
}

// Leg is how long a Travel Leg aboard takes at a speed in miles a day: its minutes under way, and the
// travel days they span.
func Leg(distanceMi, milesPerDay int, kind string) (int, int) {
	day := HoursPerDay(kind) * 60
	minutes := (distanceMi*day + milesPerDay - 1) / milesPerDay
	return minutes, (minutes + day - 1) / day
}

// Elapsed is how long a Travel Leg aboard takes on the clock: its minutes under way, and between its
// travel days the hours a land vehicle stands still. A ship sails through the night.
func Elapsed(minutes, days int, kind string) int {
	return minutes + max(0, days-1)*(hoursUnderWay-HoursPerDay(kind))*60
}
