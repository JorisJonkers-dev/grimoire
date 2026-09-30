package hex

// Cover is how much of a target intervening things hide.
type Cover int

// Cover levels, in increasing order.
const (
	NoCover Cover = iota
	HalfCover
	ThreeQuartersCover
	TotalCover
)

// ACBonus is what cover adds to AC and Dexterity saves.
func (c Cover) ACBonus() int {
	switch c {
	case HalfCover:
		return 2
	case ThreeQuartersCover:
		return 5
	case NoCover, TotalCover:
		return 0
	}
	return 0
}

// Cell is one hex of a local map.
type Cell struct {
	Difficult   bool
	Blocked     bool
	BlocksSight bool
	ElevationFt int
	Cover       Cover
}

// Occupant says who stands in a hex, from the mover's point of view.
type Occupant int

// Occupants.
const (
	Ally Occupant = iota + 1
	Enemy
)

// Grid is the part of a map the rules look at. Hexes missing from Cells are off the map.
type Grid struct {
	Cells     map[Coord]Cell
	Occupants map[Coord]Occupant
}
