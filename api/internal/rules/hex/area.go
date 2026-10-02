package hex

import (
	"math"
	"sort"
)

// Shape is an area of effect's template.
type Shape string

// Shapes. A sphere or cylinder is centred on its point; an emanation surrounds its origin and leaves it
// out; a ring is the hexes exactly its size from its point; a cone, cube or line starts at its origin
// and extends toward the aim point; a wall is a line that includes the hex it starts on.
const (
	SphereArea    Shape = "sphere"
	CylinderArea  Shape = "cylinder"
	EmanationArea Shape = "emanation"
	RingArea      Shape = "ring"
	ConeArea      Shape = "cone"
	CubeArea      Shape = "cube"
	LineArea      Shape = "line"
	WallArea      Shape = "wall"
)

// feet places a hex centre on a plane where neighbouring centres are 5 feet apart.
func feet(c Coord) (float64, float64) {
	return FeetPerHex * (float64(c.Q) + float64(c.R)/2), FeetPerHex * math.Sqrt(3) / 2 * float64(c.R)
}

const epsilon = 1e-9

// Area rasterises a template of sizeFt to the hexes it covers. A directional shape aimed at its own
// origin covers nothing.
func Area(shape Shape, origin, aim Coord, sizeFt int) []Coord {
	reach := sizeFt/FeetPerHex + 1
	var out []Coord
	for q := -reach; q <= reach; q++ {
		for r := -reach; r <= reach; r++ {
			c := origin.Add(Coord{Q: q, R: r})
			if covers(shape, origin, aim, c, float64(sizeFt)) {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Q < out[j].Q || (out[i].Q == out[j].Q && out[i].R < out[j].R) })
	return out
}

func covers(shape Shape, origin, aim, c Coord, size float64) bool {
	d := float64(Distance(origin, c) * FeetPerHex)
	switch shape {
	case SphereArea, CylinderArea:
		return d <= size
	case EmanationArea:
		return d <= size && c != origin
	case RingArea:
		return size > 0 && d == size
	case WallArea:
		if c == origin {
			return aim != origin
		}
	case ConeArea, CubeArea, LineArea:
	}
	ox, oy := feet(origin)
	ax, ay := feet(aim)
	cx, cy := feet(c)
	ux, uy := ax-ox, ay-oy
	norm := math.Hypot(ux, uy)
	if norm == 0 {
		return false
	}
	along := ((cx-ox)*ux + (cy-oy)*uy) / norm
	across := math.Abs((cx-ox)*uy-(cy-oy)*ux) / norm
	if along <= epsilon || along > size+epsilon {
		return false
	}
	switch shape {
	case ConeArea:
		return across <= along/2+epsilon
	case CubeArea:
		return across <= size/2+epsilon
	case LineArea, WallArea, SphereArea, CylinderArea, EmanationArea, RingArea:
	}
	return across <= FeetPerHex/2+epsilon
}

// Push is the hex next to at that lies straight away from from, or straight toward it: the neighbour
// most in line with the two. Nothing moves a hex away from or toward itself.
func Push(from, at Coord, toward bool) Coord {
	if from == at {
		return at
	}
	fx, fy := feet(from)
	ax, ay := feet(at)
	dx, dy := ax-fx, ay-fy
	if toward {
		dx, dy = -dx, -dy
	}
	best, score := at, math.Inf(-1)
	for _, n := range at.Neighbors() {
		nx, ny := feet(n)
		if s := (nx-ax)*dx + (ny-ay)*dy; s > score {
			best, score = n, s
		}
	}
	return best
}
