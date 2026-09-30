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
			x := l.Origin.X - 3*l.Size + float64(i*37%400)/400*6*l.Size
			y := l.Origin.Y - 3*l.Size + float64(i*53%400)/400*6*l.Size
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

// TestGoldenGeometry pins the Go geometry to the fixture the web client is tested against.
func TestGoldenGeometry(t *testing.T) {
	got, err := json.MarshalIndent(build(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(fixture, append(got, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got)+"\n" {
		t.Fatal("geometry changed; rerun with -update and check the web client agrees")
	}
}
