package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Blobs is the object storage port.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// MaxImageBytes is the largest picture a player may upload.
const MaxImageBytes = 10 << 20

// sniff recognises PNG, JPEG and WebP by their magic bytes; the declared type is never trusted.
func sniff(data []byte) (string, string, bool) {
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

// SetImage stores a portrait or token icon. The owner or a DM, never during Combat.
func (s *Characters) SetImage(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind, data []byte) error {
	if len(data) == 0 || len(data) > MaxImageBytes {
		return refuse("pictures must be at most 10 MB")
	}
	contentType, ext, ok := sniff(data)
	if !ok {
		return refuse("pictures must be PNG, JPEG or WebP")
	}
	if _, err := s.editable(ctx, c, id, ch); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	img := &domain.Image{Key: "sha256/" + hex.EncodeToString(sum[:]) + "." + ext, Type: contentType}
	if err := s.Blobs.Put(ctx, img.Key, img.Type, data); err != nil {
		return err
	}
	return s.Repo.SetCharacterImage(ctx, id, ch, kind, img, s.Now())
}

// ClearToken goes back to initials for the token.
func (s *Characters) ClearToken(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) error {
	if _, err := s.editable(ctx, c, id, ch); err != nil {
		return err
	}
	return s.Repo.SetCharacterImage(ctx, id, ch, domain.TokenIcon, nil, s.Now())
}

// Image returns a Character's portrait or token icon to a Member of its Campaign.
func (s *Characters) Image(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind) (domain.Image, []byte, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return domain.Image{}, nil, err
	}
	stored, err := s.Repo.Character(ctx, id, ch)
	if err != nil {
		return domain.Image{}, nil, err
	}
	img := stored.Portrait
	if kind == domain.TokenIcon {
		img = stored.Token
	}
	if img == nil {
		return domain.Image{}, nil, domain.ErrNotFound
	}
	data, err := s.Blobs.Get(ctx, img.Key)
	if err != nil {
		return domain.Image{}, nil, err
	}
	return *img, data, nil
}
