package hex_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func TestLineOfSightAndCover(t *testing.T) {
	t.Parallel()
	g := field(6)
	from, to := c(-3, 0), c(3, 0)
	if s := hex.LineOfSight(g, from, to); !s.Visible || s.Cover != hex.NoCover {
		t.Fatalf("open field = %+v", s)
	}
	cases := []struct {
		name string
		at   hex.Coord
		cell hex.Cell
		occ  hex.Occupant
		want hex.Sight
	}{
		{"wall", c(0, 0), hex.Cell{BlocksSight: true}, 0, hex.Sight{Visible: false, Cover: hex.TotalCover}},
		{"low wall", c(0, 0), hex.Cell{Cover: hex.HalfCover}, 0, hex.Sight{Visible: true, Cover: hex.HalfCover}},
		{"arrow slit", c(1, 0), hex.Cell{Cover: hex.ThreeQuartersCover}, 0, hex.Sight{Visible: true, Cover: hex.ThreeQuartersCover}},
		{"total", c(1, 0), hex.Cell{Cover: hex.TotalCover}, 0, hex.Sight{Visible: false, Cover: hex.TotalCover}},
		{"creature", c(-1, 0), hex.Cell{}, hex.Ally, hex.Sight{Visible: true, Cover: hex.HalfCover}},
		{"ridge", c(0, 0), hex.Cell{ElevationFt: 6}, 0, hex.Sight{Visible: false, Cover: hex.TotalCover}},
		{"knoll", c(0, 0), hex.Cell{ElevationFt: 5}, 0, hex.Sight{Visible: true, Cover: hex.NoCover}},
	}
	for _, tc := range cases {
		grid := field(6)
		grid.Cells[tc.at] = tc.cell
		if tc.occ != 0 {
			grid.Occupants[tc.at] = tc.occ
		}
		if got := hex.LineOfSight(grid, from, to); got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
	high := field(6)
	high.Cells[from] = hex.Cell{ElevationFt: 20}
	high.Cells[c(0, 0)] = hex.Cell{ElevationFt: 10}
	if s := hex.LineOfSight(high, from, to); !s.Visible {
		t.Fatalf("seeing over a ridge from a tower = %+v", s)
	}
	end := field(6)
	end.Cells[to] = hex.Cell{BlocksSight: true, Cover: hex.TotalCover}
	end.Occupants[from] = hex.Ally
	if s := hex.LineOfSight(end, from, to); !s.Visible || s.Cover != hex.NoCover {
		t.Fatalf("endpoints do not cover themselves = %+v", s)
	}
	if s := hex.LineOfSight(g, from, from); !s.Visible {
		t.Fatalf("self = %+v", s)
	}
}
