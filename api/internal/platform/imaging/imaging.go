// Package imaging reads map pictures and hides what players have never seen before they are sent.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // decoders register themselves
	"image/png"

	_ "golang.org/x/image/webp" // decoder registers itself

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// ErrUnreadable is returned for data that is not a PNG, JPEG or WebP picture.
var ErrUnreadable = errors.New("imaging: unreadable picture")

// MaxPixels caps map size so masking stays quick: 6000 × 6000.
const MaxPixels = 36_000_000

// Info is what a picture is without decoding it.
type Info struct {
	Width       int
	Height      int
	ContentType string
}

// Inspect reads a picture's dimensions and type without decoding it.
func Inspect(data []byte) (Info, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Info{}, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Info{}, fmt.Errorf("%w: larger than %d pixels", ErrUnreadable, MaxPixels)
	}
	return Info{Width: cfg.Width, Height: cfg.Height, ContentType: "image/" + format}, nil
}

// Mask returns a PNG of the picture where every pixel outside a kept hex is black. The hidden parts of
// the map never leave the server.
func Mask(data []byte, l hex.Layout, keep map[hex.Coord]bool) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	b := src.Bounds()
	out := image.NewRGBA(b)
	draw.Draw(out, b, &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if keep[l.FromPixel(hex.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})] {
				out.Set(x, y, src.At(x, y))
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, out) // encoding an in-memory RGBA picture cannot fail
	return buf.Bytes(), nil
}

// Cells returns every hex whose centre lies inside a picture of the given size.
func Cells(l hex.Layout, width, height int) []hex.Coord {
	corners := []hex.Coord{
		l.FromPixel(hex.Point{X: 0, Y: 0}), l.FromPixel(hex.Point{X: float64(width), Y: 0}),
		l.FromPixel(hex.Point{X: 0, Y: float64(height)}), l.FromPixel(hex.Point{X: float64(width), Y: float64(height)}),
	}
	minQ, maxQ, minR, maxR := corners[0].Q, corners[0].Q, corners[0].R, corners[0].R
	for _, c := range corners[1:] {
		minQ, maxQ, minR, maxR = min(minQ, c.Q), max(maxQ, c.Q), min(minR, c.R), max(maxR, c.R)
	}
	var out []hex.Coord
	for q := minQ - 1; q <= maxQ+1; q++ {
		for r := minR - 1; r <= maxR+1; r++ {
			p := l.ToPixel(hex.Coord{Q: q, R: r})
			if p.X >= 0 && p.Y >= 0 && p.X < float64(width) && p.Y < float64(height) {
				out = append(out, hex.Coord{Q: q, R: r})
			}
		}
	}
	return out
}
