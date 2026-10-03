// Package atlas holds the maps that ship with Grimoire.
package atlas

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"
)

// The Default World's picture: its size, and the terrain it is painted in, from deep sea to snow.
const (
	worldWidth  = 1600
	worldHeight = 1000
)

var terrain = []struct { //nolint:gochecknoglobals // a fixed table
	below float64
	paint color.RGBA
}{
	{0.42, color.RGBA{R: 38, G: 74, B: 112, A: 255}},
	{0.50, color.RGBA{R: 58, G: 104, B: 142, A: 255}},
	{0.53, color.RGBA{R: 214, G: 198, B: 150, A: 255}},
	{0.62, color.RGBA{R: 132, G: 164, B: 96, A: 255}},
	{0.70, color.RGBA{R: 84, G: 128, B: 78, A: 255}},
	{0.78, color.RGBA{R: 142, G: 132, B: 100, A: 255}},
	{0.86, color.RGBA{R: 118, G: 108, B: 98, A: 255}},
	{math.Inf(1), color.RGBA{R: 236, G: 236, B: 232, A: 255}},
}

// lattice is a fixed pseudo-random height in [0, 1) at a whole point of the plane.
func lattice(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263 //nolint:gosec // wraps on purpose
	h = (h ^ (h >> 13)) * 1274126177
	return float64((h^(h>>16))&0xffff) / 0x10000
}

// rolling is smooth noise: the lattice heights around a point, blended.
func rolling(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	ix, iy := int(x0), int(y0)
	top := lattice(ix, iy)*(1-fx) + lattice(ix+1, iy)*fx
	bottom := lattice(ix, iy+1)*(1-fx) + lattice(ix+1, iy+1)*fx
	return top*(1-fy) + bottom*fy
}

// height is the land's height at a pixel: rolling noise at four scales, sinking into the sea towards
// the edges of the picture.
func height(px, py int) float64 {
	x, y := float64(px)/worldWidth, float64(py)/worldHeight
	h, weight, scale := 0.0, 0.5, 5.0
	for range 4 {
		h += weight * rolling(x*scale*worldWidth/worldHeight, y*scale)
		weight, scale = weight/2, scale*2
	}
	edge := math.Max(math.Abs(x-0.5), math.Abs(y-0.5)) * 2
	return 0.9*h/0.9375 + 0.2 - 0.75*math.Pow(edge, 2.5)
}

// DefaultWorld is the painted world Map that ships with Grimoire, as a PNG: original land in a sea,
// the same every time.
var DefaultWorld = sync.OnceValue(func() []byte { //nolint:gochecknoglobals // painted once
	palette := make(color.Palette, len(terrain))
	for i, t := range terrain {
		palette[i] = t.paint
	}
	img := image.NewPaletted(image.Rect(0, 0, worldWidth, worldHeight), palette)
	for y := range worldHeight {
		for x := range worldWidth {
			h, band := height(x, y), 0
			for h >= terrain[band].below {
				band++
			}
			img.SetColorIndex(x, y, uint8(band)) //nolint:gosec // one of eight
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // a buffer takes every write
	return buf.Bytes()
})
