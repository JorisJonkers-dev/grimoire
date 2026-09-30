// Package travel holds the overland rules: travel pace and how long a Travel Leg takes.
package travel

// Pace is how fast the party travels overland.
type Pace int

// Paces.
const (
	Slow Pace = iota
	Normal
	Fast
)

// HoursPerDay is how long the party travels in one day without a forced march.
const HoursPerDay = 8

// SightHexes is how far around a location the party can see when it arrives there.
const SightHexes = 2

// ParsePace reads a pace by name.
func ParsePace(s string) (Pace, bool) {
	switch s {
	case "slow":
		return Slow, true
	case "normal":
		return Normal, true
	case "fast":
		return Fast, true
	}
	return Normal, false
}

// String names the pace.
func (p Pace) String() string {
	return [...]string{"slow", "normal", "fast"}[p]
}

// MilesPerHour is the pace's speed.
func (p Pace) MilesPerHour() int {
	return [...]int{2, 3, 4}[p]
}

// Leg is how long one Travel Leg takes: its minutes on the road and the travel days they span.
type Leg struct {
	DistanceMi int
	Pace       Pace
	Minutes    int
	Days       int
}

// Plan works out a Travel Leg of distanceMi miles at pace p.
func Plan(distanceMi int, p Pace) Leg {
	minutes := distanceMi * 60 / p.MilesPerHour()
	day := HoursPerDay * 60
	return Leg{DistanceMi: distanceMi, Pace: p, Minutes: minutes, Days: (minutes + day - 1) / day}
}
