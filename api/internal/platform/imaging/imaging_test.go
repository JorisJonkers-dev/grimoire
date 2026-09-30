package imaging_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func picture(t *testing.T, w, h int, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 180, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngEncode(b *bytes.Buffer, img image.Image) error { return png.Encode(b, img) }

func jpegEncode(b *bytes.Buffer, img image.Image) error { return jpeg.Encode(b, img, nil) }

func TestSizeAndCells(t *testing.T) {
	t.Parallel()
	info, err := imaging.Inspect(picture(t, 120, 80, pngEncode))
	if err != nil || info.Width != 120 || info.Height != 80 || info.ContentType != "image/png" {
		t.Fatalf("info = %+v %v", info, err)
	}
	if info, _ := imaging.Inspect(picture(t, 4, 4, jpegEncode)); info.ContentType != "image/jpeg" {
		t.Fatalf("jpeg = %+v", info)
	}
	if _, err := imaging.Inspect([]byte("nope")); !errors.Is(err, imaging.ErrUnreadable) {
		t.Fatalf("bad data = %v", err)
	}
	huge := picture(t, 1, 1, pngEncode)
	// Patch the PNG header to claim 7000 × 7000 pixels.
	huge[16], huge[17], huge[18], huge[19] = 0, 0, 0x1b, 0x58
	huge[20], huge[21], huge[22], huge[23] = 0, 0, 0x1b, 0x58
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, err := imaging.Inspect(huge); !errors.Is(err, imaging.ErrUnreadable) {
		t.Fatalf("huge = %v", err)
	}
	l := hex.Layout{Size: 20, Origin: hex.Point{X: 20, Y: 20}}
	cells := imaging.Cells(l, 120, 80)
	if len(cells) == 0 {
		t.Fatal("no cells")
	}
	for _, c := range cells {
		p := l.ToPixel(c)
		if p.X < 0 || p.Y < 0 || p.X >= 120 || p.Y >= 80 {
			t.Fatalf("cell %v centre %v outside the picture", c, p)
		}
	}
}

func TestMaskBlacksOutEverythingNotKept(t *testing.T) {
	t.Parallel()
	l := hex.Layout{Size: 20, Origin: hex.Point{X: 30, Y: 30}}
	for name, data := range map[string][]byte{"png": picture(t, 100, 60, pngEncode), "jpeg": picture(t, 100, 60, jpegEncode)} {
		out, err := imaging.Mask(data, l, map[hex.Coord]bool{{Q: 0, R: 0}: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		img, err := png.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		centre := img.At(30, 30)
		if r, _, _, _ := centre.RGBA(); r == 0 {
			t.Fatalf("%s: kept hex went black", name)
		}
		far := l.ToPixel(hex.Coord{Q: 1, R: 0})
		if r, g, b, _ := img.At(int(far.X), int(far.Y)).RGBA(); r != 0 || g != 0 || b != 0 {
			t.Fatalf("%s: unseen hex shows %v", name, img.At(int(far.X), int(far.Y)))
		}
	}
	if _, err := imaging.Mask([]byte("nope"), l, nil); !errors.Is(err, imaging.ErrUnreadable) {
		t.Fatalf("bad data = %v", err)
	}
}
