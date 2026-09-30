package hex_test

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

var update = flag.Bool("update", false, "rewrite the shared geometry fixture")

const fixture = "../../../../fixtures/hex-geometry.json"

type golden struct {
	Layouts []goldenLayout `json:"layouts"`
	Lines   []goldenLine   `json:"lines"`
}

type goldenLayout struct {
	Size    float64       `json:"size"`
	OriginX float64       `json:"originX"`
	OriginY float64       `json:"originY"`
	Centres []goldenPixel `json:"centres"`
	Picks   []goldenPixel `json:"picks"`
}

type goldenPixel struct {
	Q int     `json:"q"`
	R int     `json:"r"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type goldenLine struct {
	From     [2]int   `json:"from"`
	To       [2]int   `json:"to"`
	Distance int      `json:"distance"`
	Hexes    [][2]int `json:"hexes"`
}

func build() golden {
	var g golden
	for _, l := range []hex.Layout{{Size: 36, Origin: hex.Point{X: 0, Y: 0}}, {Size: 24.5, Origin: hex.Point{X: 100, Y: 50}}} {
		gl := goldenLayout{Size: l.Size, OriginX: l.Origin.X, OriginY: l.Origin.Y}
		for q := -3; q <= 3; q++ {
			for r := -3; r <= 3; r++ {
				p := l.ToPixel(hex.Coord{Q: q, R: r})
				gl.Centres = append(gl.Centres, goldenPixel{Q: q, R: r, X: math.Round(p.X*1000) / 1000, Y: math.Round(p.Y*1000) / 1000})
			}
		}
		for i := range 40 {
			want := hex.Coord{Q: i%7 - 3, R: i/7%7 - 3}
			centre := l.ToPixel(want)
			angle := float64(i) * 0.7
			x := math.Round((centre.X+0.6*l.Size*math.Cos(angle))*1000) / 1000
			y := math.Round((centre.Y+0.6*l.Size*math.Sin(angle))*1000) / 1000
			c := l.FromPixel(hex.Point{X: x, Y: y})
			gl.Picks = append(gl.Picks, goldenPixel{Q: c.Q, R: c.R, X: x, Y: y})
		}
		g.Layouts = append(g.Layouts, gl)
	}
	for _, pair := range [][2]hex.Coord{{{Q: 0, R: 0}, {Q: 3, R: 0}}, {{Q: 0, R: 0}, {Q: 2, R: -4}}, {{Q: -2, R: 1}, {Q: 3, R: -2}}, {{Q: 1, R: 1}, {Q: 1, R: 1}}} {
		gl := goldenLine{From: [2]int{pair[0].Q, pair[0].R}, To: [2]int{pair[1].Q, pair[1].R}, Distance: hex.Distance(pair[0], pair[1])}
		for _, c := range hex.Line(pair[0], pair[1]) {
			gl.Hexes = append(gl.Hexes, [2]int{c.Q, c.R})
		}
		g.Lines = append(g.Lines, gl)
	}
	return g
}

// TestGoldenGeometry pins the Go geometry to the fixture the web client is tested against. It compares
// values rather than bytes, because floating point differs in the last digits between CPUs.
func TestGoldenGeometry(t *testing.T) {
	if *update {
		got, err := json.MarshalIndent(build(), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, append(got, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var want golden
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for _, gl := range want.Layouts {
		checkLayout(t, gl)
	}
	for _, gl := range want.Lines {
		checkLine(t, gl)
	}
}

func checkLayout(t *testing.T, gl goldenLayout) {
	t.Helper()
	l := hex.Layout{Size: gl.Size, Origin: hex.Point{X: gl.OriginX, Y: gl.OriginY}}
	for _, p := range gl.Centres {
		got := l.ToPixel(hex.Coord{Q: p.Q, R: p.R})
		if math.Abs(got.X-p.X) > 1e-3 || math.Abs(got.Y-p.Y) > 1e-3 {
			t.Errorf("centre of %d,%d = %v, fixture %v,%v", p.Q, p.R, got, p.X, p.Y)
		}
	}
	for _, p := range gl.Picks {
		if got := l.FromPixel(hex.Point{X: p.X, Y: p.Y}); got != (hex.Coord{Q: p.Q, R: p.R}) {
			t.Errorf("pick %v,%v = %v, fixture %d,%d", p.X, p.Y, got, p.Q, p.R)
		}
	}
}

func checkLine(t *testing.T, gl goldenLine) {
	t.Helper()
	{
		a, b := hex.Coord{Q: gl.From[0], R: gl.From[1]}, hex.Coord{Q: gl.To[0], R: gl.To[1]}
		line := hex.Line(a, b)
		if hex.Distance(a, b) != gl.Distance || len(line) != len(gl.Hexes) {
			t.Fatalf("line %v-%v = %v", a, b, line)
		}
		for i, c := range line {
			if [2]int{c.Q, c.R} != gl.Hexes[i] {
				t.Errorf("line %v-%v step %d = %v, fixture %v", a, b, i, c, gl.Hexes[i])
			}
		}
	}
}
