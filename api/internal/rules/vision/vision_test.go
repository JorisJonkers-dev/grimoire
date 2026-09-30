package vision_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

func c(q, r int) hex.Coord { return hex.Coord{Q: q, R: r} }

// room is a line of hexes from q=-4 to q=4 with a wall at q=0 unless open.
func room(open bool) hex.Grid {
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: map[hex.Coord]hex.Occupant{}}
	for q := -4; q <= 4; q++ {
		g.Cells[c(q, 0)] = hex.Cell{}
	}
	if !open {
		g.Cells[c(0, 0)] = hex.Cell{BlocksSight: true}
	}
	return g
}

func TestBrightAndDimAmbientShowEverythingInSight(t *testing.T) {
	t.Parallel()
	for _, a := range []vision.Ambient{vision.Bright, vision.Dim} {
		s := vision.Scene{Grid: room(false), Ambient: a}
		seen := s.Visible([]vision.Viewer{{At: c(-3, 0)}})
		if !seen[c(-1, 0)] || !seen[c(0, 0)] || seen[c(1, 0)] || seen[c(4, 0)] {
			t.Fatalf("ambient %d sees %v", a, seen)
		}
	}
}

func TestDarknessNeedsLightOrDarkvision(t *testing.T) {
	t.Parallel()
	dark := vision.Scene{Grid: room(true), Ambient: vision.Dark}
	seen := dark.Visible([]vision.Viewer{{At: c(-3, 0)}})
	if len(seen) != 1 || !seen[c(-3, 0)] {
		t.Fatalf("pitch dark sees %v", seen)
	}
	seen = dark.Visible([]vision.Viewer{{At: c(-3, 0), DarkvisionFt: 10}})
	if len(seen) != 4 || !seen[c(-4, 0)] || !seen[c(-1, 0)] || seen[c(0, 0)] {
		t.Fatalf("darkvision 10 ft sees %v", seen)
	}
	torch := vision.Scene{Grid: room(true), Ambient: vision.Dark, Lights: []vision.Light{{At: c(3, 0), BrightFt: 5, DimFt: 10}}}
	seen = torch.Visible([]vision.Viewer{{At: c(-3, 0)}})
	if !seen[c(1, 0)] || !seen[c(4, 0)] || seen[c(0, 0)] || !seen[c(-3, 0)] {
		t.Fatalf("a distant torch lights %v", seen)
	}
	lamp := vision.Scene{Grid: room(true), Ambient: vision.Dark, Lights: []vision.Light{{At: c(3, 0), BrightFt: 10}}}
	if seen := lamp.Visible([]vision.Viewer{{At: c(-3, 0)}}); !seen[c(1, 0)] || seen[c(0, 0)] {
		t.Fatalf("bright-only light lights %v", seen)
	}
}

func TestWallsStopLightAndSight(t *testing.T) {
	t.Parallel()
	s := vision.Scene{Grid: room(false), Ambient: vision.Dark, Lights: []vision.Light{{At: c(-2, 0), BrightFt: 30, DimFt: 30}}}
	lit := s.Lit()
	if !lit[c(-1, 0)] || !lit[c(0, 0)] || lit[c(1, 0)] {
		t.Fatalf("lit = %v", lit)
	}
	behind := s.Visible([]vision.Viewer{{At: c(3, 0), DarkvisionFt: 60}})
	if behind[c(-1, 0)] || !behind[c(1, 0)] {
		t.Fatalf("seeing through a wall: %v", behind)
	}
}

func TestPartyVisionIsTheUnion(t *testing.T) {
	t.Parallel()
	s := vision.Scene{Grid: room(false), Ambient: vision.Bright}
	seen := s.Visible([]vision.Viewer{{At: c(-3, 0)}, {At: c(3, 0)}, {At: c(9, 9)}})
	for q := -4; q <= 4; q++ {
		if !seen[c(q, 0)] {
			t.Fatalf("union misses %d: %v", q, seen)
		}
	}
	if len(s.Visible(nil)) != 0 {
		t.Fatal("nobody sees something")
	}
}
