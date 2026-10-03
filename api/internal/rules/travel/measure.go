package travel

import (
	"math"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// Measured is the length of a route on a world Map.
type Measured struct {
	Hexes int
	Miles float64
}

// Measure walks a route through its waypoints: the hexes from each to the next, and the miles they
// cover on a Map where one hex is milesPerHex across.
func Measure(waypoints []hex.Coord, milesPerHex float64) Measured {
	hexes := 0
	for i := 1; i < len(waypoints); i++ {
		hexes += hex.Distance(waypoints[i-1], waypoints[i])
	}
	return Measured{Hexes: hexes, Miles: float64(hexes) * milesPerHex}
}

// Time is how long measured miles take at a pace: the minutes on the road, rounded up to a whole
// minute, and the travel days they span.
func Time(miles float64, p Pace) (int, int) {
	// A hair is taken off before rounding up, so that miles that divide evenly are not a minute long.
	minutes := int(math.Ceil(miles*60/float64(p.MilesPerHour()) - 1e-9))
	day := HoursPerDay * 60
	return minutes, (minutes + day - 1) / day
}
