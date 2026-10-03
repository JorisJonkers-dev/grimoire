package hex

import "math"

// Across is how far apart neighbouring cells of a grid of a Size are, centre to centre: √3 times the
// Size. A grid drawn as squares has squares that wide, so a cell is the same distance either way.
func Across(size float64) float64 {
	return size * math.Sqrt(3)
}

func apart(a, b Point) float64 {
	return math.Hypot(b.X-a.X, b.Y-a.Y)
}

// Calibrate is the Size of the grid in which two points on a picture are so many cells apart. It is
// false for points that coincide and for a count that is not a positive number.
func Calibrate(a, b Point, cells float64) (float64, bool) {
	d := apart(a, b)
	if d == 0 || !(cells > 0) || math.IsInf(cells, 1) {
		return 0, false
	}
	return d / cells / Across(1), true
}

// Cells measures the distance between two points on a picture in cells of a grid of a Size.
func Cells(size float64, a, b Point) float64 {
	if !(size > 0) {
		return 0
	}
	return apart(a, b) / Across(size)
}
