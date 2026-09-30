// Package vision decides what the party can see: light, darkvision and line of sight, shared by all.
package vision

import "github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"

// Ambient is the light that fills a map everywhere.
type Ambient int

// Ambient light levels.
const (
	Bright Ambient = iota
	Dim
	Dark
)

// Light is a source that lights hexes it can reach: brightly up to BrightFt, dimly up to DimFt.
type Light struct {
	At       hex.Coord
	BrightFt int
	DimFt    int
}

// Viewer is a party member's eyes.
type Viewer struct {
	At           hex.Coord
	DarkvisionFt int
}

// Scene is everything vision depends on. Occupants are ignored: creatures do not block sight.
type Scene struct {
	Grid    hex.Grid
	Ambient Ambient
	Lights  []Light
}

func (s Scene) walls() hex.Grid {
	return hex.Grid{Cells: s.Grid.Cells, Occupants: nil}
}

func inRange(a, b hex.Coord, feet int) bool {
	return hex.Distance(a, b)*hex.FeetPerHex <= feet
}

// Lit returns every hex some light reaches, or every hex when the ambient light is enough to see by.
func (s Scene) Lit() map[hex.Coord]bool {
	lit := map[hex.Coord]bool{}
	g := s.walls()
	for c := range s.Grid.Cells {
		if s.Ambient != Dark {
			lit[c] = true
			continue
		}
		for _, l := range s.Lights {
			if inRange(l.At, c, max(l.BrightFt, l.DimFt)) && hex.LineOfSight(g, l.At, c).Visible {
				lit[c] = true
				break
			}
		}
	}
	return lit
}

// Visible is the party's shared vision: every hex any viewer has a clear line to and can see by light
// or darkvision. Darkness beyond every sense is not visible at all.
func (s Scene) Visible(viewers []Viewer) map[hex.Coord]bool {
	lit := s.Lit()
	g := s.walls()
	out := map[hex.Coord]bool{}
	for _, v := range viewers {
		if _, onMap := s.Grid.Cells[v.At]; !onMap {
			continue
		}
		out[v.At] = true
		for c := range s.Grid.Cells {
			if out[c] || (!lit[c] && !inRange(v.At, c, v.DarkvisionFt)) {
				continue
			}
			if hex.LineOfSight(g, v.At, c).Visible {
				out[c] = true
			}
		}
	}
	return out
}
