// Package hex is the pure hex grid: axial pointy-top coordinates, 5 ft per hex.
package hex

import "math"

// FeetPerHex is the width of one hex.
const FeetPerHex = 5

// Coord is an axial hex coordinate.
type Coord struct {
	Q int
	R int
}

// Add returns c moved by d.
func (c Coord) Add(d Coord) Coord {
	return Coord{Q: c.Q + d.Q, R: c.R + d.R}
}

// directions lists the six neighbours, clockwise from east.
func directions() [6]Coord {
	return [6]Coord{{Q: 1, R: 0}, {Q: 1, R: -1}, {Q: 0, R: -1}, {Q: -1, R: 0}, {Q: -1, R: 1}, {Q: 0, R: 1}}
}

// Neighbors returns the six adjacent hexes.
func (c Coord) Neighbors() [6]Coord {
	var out [6]Coord
	for i, d := range directions() {
		out[i] = c.Add(d)
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Distance counts hexes between two coordinates.
func Distance(a, b Coord) int {
	dq, dr := a.Q-b.Q, a.R-b.R
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
}

// Disk lists every hex at most radius hexes from centre, centre included.
func Disk(centre Coord, radius int) []Coord {
	var out []Coord
	for q := -radius; q <= radius; q++ {
		for r := max(-radius, -q-radius); r <= min(radius, radius-q); r++ {
			out = append(out, centre.Add(Coord{Q: q, R: r}))
		}
	}
	return out
}

// Point is a position in pixels.
type Point struct {
	X float64
	Y float64
}

// Layout maps hexes to pixels: Size is centre to corner, Origin the centre of hex (0, 0).
type Layout struct {
	Size   float64
	Origin Point
}

// ToPixel returns the centre of a hex.
func (l Layout) ToPixel(c Coord) Point {
	x := l.Size * (math.Sqrt(3)*float64(c.Q) + math.Sqrt(3)/2*float64(c.R))
	y := l.Size * 1.5 * float64(c.R)
	return Point{X: x + l.Origin.X, Y: y + l.Origin.Y}
}

// FromPixel returns the hex containing a pixel.
func (l Layout) FromPixel(p Point) Coord {
	px, py := (p.X-l.Origin.X)/l.Size, (p.Y-l.Origin.Y)/l.Size
	q := math.Sqrt(3)/3*px - py/3
	r := 2.0 / 3 * py
	return round(q, r)
}

// round snaps fractional axial coordinates to the nearest hex.
func round(fq, fr float64) Coord {
	fs := -fq - fr
	q, r, s := math.Round(fq), math.Round(fr), math.Round(fs)
	dq, dr, ds := math.Abs(q-fq), math.Abs(r-fr), math.Abs(s-fs)
	switch {
	case dq > dr && dq > ds:
		q = -r - s
	case dr > ds:
		r = -q - s
	}
	return Coord{Q: int(q), R: int(r)}
}

// Line returns every hex a straight line from a to b passes through, both ends included.
// The endpoints are nudged by a hair so a line along a hex edge picks one side consistently.
func Line(a, b Coord) []Coord {
	n := Distance(a, b)
	out := make([]Coord, 0, n+1)
	const eps = 1e-6
	aq, ar := float64(a.Q)+eps, float64(a.R)+eps
	bq, br := float64(b.Q)+eps, float64(b.R)+eps
	for i := 0; i <= n; i++ {
		t := 0.0
		if n > 0 {
			t = float64(i) / float64(n)
		}
		out = append(out, round(aq+(bq-aq)*t, ar+(br-ar)*t))
	}
	return out
}
