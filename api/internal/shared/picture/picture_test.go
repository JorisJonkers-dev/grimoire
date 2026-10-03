package picture_test

import (
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/picture"
)

func TestSniffTellsPicturesByTheirBytes(t *testing.T) {
	t.Parallel()
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBP"), 1, 2, 3)
	for name, c := range map[string]struct {
		data      []byte
		kind, ext string
		ok        bool
	}{
		"png":         {[]byte("\x89PNG\r\n\x1a\nrest"), "image/png", "png", true},
		"jpeg":        {[]byte{0xFF, 0xD8, 0xFF, 0xE0, 1}, "image/jpeg", "jpg", true},
		"webp":        {webp, "image/webp", "webp", true},
		"riff, short": {[]byte("RIFF\x00\x00\x00\x00WEB"), "", "", false},
		"riff, other": {[]byte("RIFF\x00\x00\x00\x00WAVEfmt "), "", "", false},
		"text":        {[]byte("<svg>"), "", "", false},
		"empty":       {nil, "", "", false},
	} {
		if kind, ext, ok := picture.Sniff(c.data); kind != c.kind || ext != c.ext || ok != c.ok {
			t.Errorf("%s = %q %q %v", name, kind, ext, ok)
		}
	}
	key := picture.Key([]byte("abc"), "png")
	if key != "sha256/ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad.png" || !strings.HasPrefix(picture.Key([]byte("abd"), "jpg"), "sha256/") || picture.Key([]byte("abd"), "jpg") == key {
		t.Fatalf("key = %s", key)
	}
}
