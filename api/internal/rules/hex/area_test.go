package hex_test

import (
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func has(cs []hex.Coord, c hex.Coord) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

func TestRoundAreas(t *testing.T) {
	t.Parallel()
	o := hex.Coord{Q: 0, R: 0}
	if got := hex.Area(hex.SphereArea, o, o, 20); len(got) != 61 || !has(got, o) || !has(got, hex.Coord{Q: 4, R: 0}) || has(got, hex.Coord{Q: 5, R: 0}) {
		t.Fatalf("20 ft sphere = %d hexes", len(got))
	}
	if got := hex.Area(hex.CylinderArea, o, o, 5); len(got) != 7 {
		t.Fatalf("5 ft cylinder = %v", got)
	}
	if got := hex.Area(hex.EmanationArea, o, o, 5); len(got) != 6 || has(got, o) {
		t.Fatalf("5 ft emanation leaves the origin out = %v", got)
	}
	if got := hex.Area(hex.SphereArea, o, o, 0); !reflect.DeepEqual(got, []hex.Coord{o}) {
		t.Fatalf("a point = %v", got)
	}
}

func TestDirectionalAreas(t *testing.T) {
	t.Parallel()
	o, east := hex.Coord{Q: 0, R: 0}, hex.Coord{Q: 1, R: 0}
	line := hex.Area(hex.LineArea, o, east, 20)
	if !reflect.DeepEqual(line, []hex.Coord{{Q: 1, R: 0}, {Q: 2, R: 0}, {Q: 3, R: 0}, {Q: 4, R: 0}}) {
		t.Fatalf("20 ft line east = %v", line)
	}
	cone := hex.Area(hex.ConeArea, o, east, 15)
	want := []hex.Coord{{Q: 1, R: 0}, {Q: 2, R: 0}, {Q: 2, R: 1}, {Q: 3, R: -1}, {Q: 3, R: 0}}
	if !reflect.DeepEqual(cone, want) {
		t.Fatalf("15 ft cone east = %v", cone)
	}
	if has(cone, o) || has(cone, hex.Coord{Q: -1, R: 0}) {
		t.Fatal("a cone starts past its origin and only goes forward")
	}
	cube := hex.Area(hex.CubeArea, o, east, 10)
	if !has(cube, hex.Coord{Q: 1, R: 0}) || !has(cube, hex.Coord{Q: 2, R: 0}) || has(cube, hex.Coord{Q: 3, R: 0}) || has(cube, o) {
		t.Fatalf("10 ft cube east = %v", cube)
	}
	if !has(cube, hex.Coord{Q: 1, R: -1}) || !has(cube, hex.Coord{Q: 0, R: 1}) {
		t.Fatalf("a cube is as wide as it is long = %v", cube)
	}
	if got := hex.Area(hex.LineArea, o, o, 30); got != nil {
		t.Fatalf("aimed at itself = %v", got)
	}
	far := hex.Area(hex.LineArea, o, hex.Coord{Q: 10, R: 0}, 10)
	if !reflect.DeepEqual(far, []hex.Coord{{Q: 1, R: 0}, {Q: 2, R: 0}}) {
		t.Fatalf("a line stops at its length, not its aim = %v", far)
	}
	diagonal := hex.Area(hex.LineArea, o, hex.Coord{Q: 0, R: 2}, 10)
	if !reflect.DeepEqual(diagonal, []hex.Coord{{Q: 0, R: 1}, {Q: 0, R: 2}}) {
		t.Fatalf("a line south-east = %v", diagonal)
	}
}
