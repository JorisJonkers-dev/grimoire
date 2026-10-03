// Package picture tells what kind of picture an upload is and names it by its content.
package picture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// MaxBytes is the largest picture anyone may upload.
const MaxBytes = 10 << 20

// Sniff tells a PNG, JPEG or WebP by its first bytes, never by its name: its content type and its file
// extension, or false for anything else.
func Sniff(data []byte) (string, string, bool) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", "png", true
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", "jpg", true
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", "webp", true
	default:
		return "", "", false
	}
}

// Key is where a picture is kept: under the hash of its content, so the same picture is kept once.
func Key(data []byte, ext string) string {
	sum := sha256.Sum256(data)
	return "sha256/" + hex.EncodeToString(sum[:]) + "." + ext
}
