package hex_test

import (
	"math"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// A cell is √3 times its Size from centre to centre.
func TestAcrossACell(t *testing.T) {
	t.Parallel()
	for size, want := range map[float64]float64{40: 40 * math.Sqrt(3), 1: math.Sqrt(3), 0: 0} {
		if got := hex.Across(size); !near(got, want) {
			t.Errorf("across a cell of size %v = %v, want %v", size, got, want)
		}
	}
	// Neighbouring hexes of a layout are exactly that far apart.
	l := hex.Layout{Size: 40, Origin: hex.Point{X: 12, Y: 34}}
	a, b := l.ToPixel(hex.Coord{Q: 2, R: 1}), l.ToPixel(hex.Coord{Q: 3, R: 1})
	if got := math.Hypot(b.X-a.X, b.Y-a.Y); !near(got, hex.Across(40)) {
		t.Errorf("neighbours are %v apart, across says %v", got, hex.Across(40))
	}
}

// Two points a known number of cells apart give the Size of the grid.
func TestCalibratingFromTwoPoints(t *testing.T) {
	t.Parallel()
	a := hex.Point{X: 100, Y: 100}
	for _, c := range []struct {
		name  string
		b     hex.Point
		cells float64
		want  float64
		ok    bool
	}{
		{"ten cells along a row", hex.Point{X: 100 + 10*40*math.Sqrt(3), Y: 100}, 10, 40, true},
		{"the same points the other way round", hex.Point{X: 100 - 10*40*math.Sqrt(3), Y: 100}, 10, 40, true},
		{"a 3-4-5 diagonal", hex.Point{X: 400, Y: 500}, 10, 50 / math.Sqrt(3), true},
		{"half a cell", hex.Point{X: 120, Y: 100}, 0.5, 40 / math.Sqrt(3), true},
		{"straight down", hex.Point{X: 100, Y: 180}, 2, 40 / math.Sqrt(3), true},
		{"the same point twice", a, 10, 0, false},
		{"no cells between", hex.Point{X: 500, Y: 100}, 0, 0, false},
		{"fewer than none", hex.Point{X: 500, Y: 100}, -3, 0, false},
		{"not a number", hex.Point{X: 500, Y: 100}, math.NaN(), 0, false},
		{"without end", hex.Point{X: 500, Y: 100}, math.Inf(1), 0, false},
	} {
		got, ok := hex.Calibrate(a, c.b, c.cells)
		if ok != c.ok || !near(got, c.want) {
			t.Errorf("%s: size %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// Measuring undoes calibrating: the points a grid was calibrated on are as many cells apart as was said.
func TestMeasuringInCells(t *testing.T) {
	t.Parallel()
	a, b := hex.Point{X: 30, Y: 70}, hex.Point{X: 630, Y: 870}
	for _, cells := range []float64{1, 7.5, 240} {
		size, ok := hex.Calibrate(a, b, cells)
		if !ok {
			t.Fatalf("%v cells did not calibrate", cells)
		}
		if got := hex.Cells(size, a, b); !near(got, cells) {
			t.Errorf("measured %v cells, calibrated on %v", got, cells)
		}
		if got := hex.Cells(size, b, a); !near(got, cells) {
			t.Errorf("measured %v cells backwards, calibrated on %v", got, cells)
		}
	}
	if got := hex.Cells(40/math.Sqrt(3), hex.Point{X: 0, Y: 0}, hex.Point{X: 0, Y: 100}); !near(got, 2.5) {
		t.Errorf("a hundred pixels of forty-pixel cells = %v", got)
	}
	if got := hex.Cells(40, a, a); got != 0 {
		t.Errorf("a point to itself = %v", got)
	}
	// A grid of no size measures nothing.
	for _, size := range []float64{0, -1} {
		if got := hex.Cells(size, a, b); got != 0 {
			t.Errorf("a grid of size %v measured %v", size, got)
		}
	}
}
