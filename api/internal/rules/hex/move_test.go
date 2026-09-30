package hex_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// field is a flat open area of the given radius around the origin.
func field(radius int) hex.Grid {
	g := hex.Grid{Cells: map[hex.Coord]hex.Cell{}, Occupants: map[hex.Coord]hex.Occupant{}}
	for q := -radius; q <= radius; q++ {
		for r := -radius; r <= radius; r++ {
			if hex.Distance(c(0, 0), c(q, r)) <= radius {
				g.Cells[c(q, r)] = hex.Cell{}
			}
		}
	}
	return g
}

func TestReachOnOpenGround(t *testing.T) {
	t.Parallel()
	g := field(6)
	reach := hex.Reachable(g, c(0, 0), hex.MoveOptions{SpeedFt: 15})
	if len(reach) != 37 {
		t.Fatalf("reachable = %d hexes, want 37", len(reach))
	}
	if reach[c(3, 0)].CostFt != 15 || !reach[c(3, 0)].CanEnd {
		t.Fatalf("three hexes east = %+v", reach[c(3, 0)])
	}
	if _, ok := reach[c(4, 0)]; ok {
		t.Fatal("four hexes reached on 15 ft")
	}
	path, ok := reach.Path(c(2, -1))
	if !ok || len(path) != 3 || path[0] != c(0, 0) || path[2] != c(2, -1) {
		t.Fatalf("path = %v %v", path, ok)
	}
	if p, ok := reach.Path(c(0, 0)); !ok || len(p) != 1 {
		t.Fatalf("path to start = %v", p)
	}
	if _, ok := reach.Path(c(9, 9)); ok {
		t.Fatal("path to an unreachable hex")
	}
}

func TestTerrainElevationAndCreatures(t *testing.T) {
	t.Parallel()
	g := field(6)
	g.Cells[c(1, 0)] = hex.Cell{Difficult: true}
	g.Cells[c(0, 1)] = hex.Cell{Blocked: true}
	g.Cells[c(-1, 0)] = hex.Cell{ElevationFt: 10}
	g.Cells[c(-1, 1)] = hex.Cell{ElevationFt: 0}
	g.Occupants[c(1, -1)] = hex.Ally
	g.Occupants[c(0, -1)] = hex.Enemy
	cases := []struct {
		to    hex.Coord
		climb bool
		cost  int
		ok    bool
	}{
		{c(1, 0), false, 10, true},
		{c(0, 1), false, 0, false},
		{c(-1, 0), false, 15, true},
		{c(-1, 0), true, 5, true},
		{c(0, -1), false, 0, false},
		{c(1, -1), false, 5, true},
		{c(9, 0), false, 0, false},
	}
	for _, tc := range cases {
		cost, ok := hex.StepCost(g, c(0, 0), tc.to, tc.climb)
		if cost != tc.cost || ok != tc.ok {
			t.Errorf("StepCost to %v (climb %v) = %d %v, want %d %v", tc.to, tc.climb, cost, ok, tc.cost, tc.ok)
		}
	}
	if cost, _ := hex.StepCost(g, c(-1, 0), c(-1, 1), false); cost != 5 {
		t.Fatalf("going down cost %d", cost)
	}
	reach := hex.Reachable(g, c(0, 0), hex.MoveOptions{SpeedFt: 10})
	if s, ok := reach[c(1, -1)]; !ok || s.CanEnd {
		t.Fatalf("ally hex = %+v %v", s, ok)
	}
	if _, ok := reach.Path(c(1, -1)); ok {
		t.Fatal("may stop in an ally's hex")
	}
	if s := reach[c(2, -2)]; s.CostFt != 10 || s.From != c(1, -1) {
		t.Fatalf("moving through the ally = %+v", s)
	}
	if _, ok := reach[c(0, -1)]; ok {
		t.Fatal("moved into an enemy")
	}
	if _, ok := reach[c(-1, 0)]; ok {
		t.Fatal("climbed 10 ft for free")
	}
	if _, ok := reach[c(2, 0)]; ok {
		t.Fatal("crossed difficult ground for 5 ft")
	}
}

func TestReachPrefersCheaperRoutes(t *testing.T) {
	t.Parallel()
	g := field(4)
	for _, x := range []hex.Coord{c(1, 0), c(1, -1)} {
		g.Cells[x] = hex.Cell{Difficult: true}
	}
	reach := hex.Reachable(g, c(0, 0), hex.MoveOptions{SpeedFt: 20})
	if s := reach[c(2, -1)]; s.CostFt != 15 {
		t.Fatalf("around the mud = %+v", s)
	}
}

func TestHighGroundBeatsAClimb(t *testing.T) {
	t.Parallel()
	g := field(4)
	g.Cells[c(0, 0)] = hex.Cell{ElevationFt: 10}
	g.Cells[c(1, 0)] = hex.Cell{ElevationFt: 0}
	g.Cells[c(1, -1)] = hex.Cell{ElevationFt: 10, Difficult: true}
	g.Cells[c(2, -1)] = hex.Cell{ElevationFt: 10}
	reach := hex.Reachable(g, c(0, 0), hex.MoveOptions{SpeedFt: 30})
	if s := reach[c(2, -1)]; s.CostFt != 15 || s.From != c(1, -1) {
		t.Fatalf("staying high = %+v", s)
	}
}
