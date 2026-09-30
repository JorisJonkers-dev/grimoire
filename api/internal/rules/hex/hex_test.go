package hex_test

import (
	"math"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func c(q, r int) hex.Coord { return hex.Coord{Q: q, R: r} }

func TestDistanceAndNeighbours(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b hex.Coord
		want int
	}{{c(0, 0), c(0, 0), 0}, {c(0, 0), c(3, 0), 3}, {c(0, 0), c(2, -4), 4}, {c(-2, 1), c(3, -2), 5}, {c(1, -3), c(-2, 2), 5}}
	for _, tc := range cases {
		if got := hex.Distance(tc.a, tc.b); got != tc.want {
			t.Errorf("Distance(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	n := c(2, -1).Neighbors()
	if n[0] != c(3, -1) || n[3] != c(1, -1) || n[5] != c(2, 0) {
		t.Fatalf("neighbours = %v", n)
	}
	for _, x := range n {
		if hex.Distance(c(2, -1), x) != 1 {
			t.Fatalf("%v is not adjacent", x)
		}
	}
}

func TestPixelRoundTrip(t *testing.T) {
	t.Parallel()
	l := hex.Layout{Size: 30, Origin: hex.Point{X: 12, Y: -7}}
	for q := -4; q <= 4; q++ {
		for r := -4; r <= 4; r++ {
			p := l.ToPixel(c(q, r))
			if got := l.FromPixel(p); got != c(q, r) {
				t.Fatalf("round trip %v -> %v -> %v", c(q, r), p, got)
			}
			nudged := l.FromPixel(hex.Point{X: p.X + l.Size*0.4, Y: p.Y + l.Size*0.3})
			if nudged != c(q, r) {
				t.Fatalf("inside %v picked %v", c(q, r), nudged)
			}
		}
	}
	p := l.ToPixel(c(1, 0))
	if math.Abs(p.X-(12+30*math.Sqrt(3))) > 1e-9 || p.Y != -7 {
		t.Fatalf("east neighbour at %v", p)
	}
	if q := l.ToPixel(c(0, 1)); math.Abs(q.Y-(-7+45)) > 1e-9 {
		t.Fatalf("south-east row at %v", q)
	}
}

func TestLines(t *testing.T) {
	t.Parallel()
	line := hex.Line(c(0, 0), c(3, 0))
	if len(line) != 4 || line[1] != c(1, 0) || line[3] != c(3, 0) {
		t.Fatalf("straight line = %v", line)
	}
	if one := hex.Line(c(2, 2), c(2, 2)); len(one) != 1 || one[0] != c(2, 2) {
		t.Fatalf("point line = %v", one)
	}
	diag := hex.Line(c(0, 0), c(2, -4))
	for i := 1; i < len(diag); i++ {
		if hex.Distance(diag[i-1], diag[i]) != 1 {
			t.Fatalf("line skips at %d: %v", i, diag)
		}
	}
}

func TestCover(t *testing.T) {
	t.Parallel()
	for cover, want := range map[hex.Cover]int{hex.NoCover: 0, hex.HalfCover: 2, hex.ThreeQuartersCover: 5, hex.TotalCover: 0} {
		if got := cover.ACBonus(); got != want {
			t.Errorf("%d: %d", cover, got)
		}
	}
	if hex.Cover(9).ACBonus() != 0 {
		t.Fatal("unknown cover")
	}
}
