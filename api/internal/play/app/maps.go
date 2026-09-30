package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MapRepository is the persistence port for Maps.
type MapRepository interface {
	InsertMap(ctx context.Context, m domain.Map, now time.Time) (domain.Map, error)
	GetMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (domain.Map, error)
	Maps(ctx context.Context, campaign uuid.UUID) ([]domain.Map, error)
	UpdateMap(ctx context.Context, m domain.Map, now time.Time) error
	LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error)
}

// Blobs is the object storage port.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// MaxMapBytes caps an uploaded map picture.
const MaxMapBytes = 25 << 20

// DefaultHexSize is where calibration starts: a hex 80 px tall.
const DefaultHexSize = 40.0

// Maps runs the Map use cases. Everything but the picture is DM prep.
type Maps struct {
	Repo    MapRepository
	Members Members
	Blobs   Blobs
	Now     func() time.Time
}

func (s *Maps) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) error {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return err
	}
	if !me.DM {
		return apperr.ErrForbidden
	}
	return nil
}

func mapName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 {
		return "", apperr.Refuse("give the map a name of up to 80 characters")
	}
	return name, nil
}

// Upload stores a map picture and starts it with a default calibration. DM only.
func (s *Maps) Upload(ctx context.Context, c caller.Caller, campaign uuid.UUID, name string, data []byte) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	name, err := mapName(name)
	if err != nil {
		return domain.Map{}, err
	}
	if len(data) > MaxMapBytes {
		return domain.Map{}, apperr.Refuse("map pictures must be at most 25 MB")
	}
	info, err := imaging.Inspect(data)
	if err != nil {
		return domain.Map{}, apperr.Refuse("map pictures must be PNG, JPEG or WebP of at most 36 megapixels")
	}
	sum := sha256.Sum256(data)
	key := "sha256/" + hex.EncodeToString(sum[:]) + "." + strings.TrimPrefix(info.ContentType, "image/")
	if err := s.Blobs.Put(ctx, key, info.ContentType, data); err != nil {
		return domain.Map{}, err
	}
	return s.Repo.InsertMap(ctx, domain.Map{
		CampaignID: campaign, Name: name, ImageKey: key, ImageType: info.ContentType, Width: info.Width, Height: info.Height,
		HexSize: DefaultHexSize, OriginX: DefaultHexSize * math.Sqrt(3) / 2, OriginY: DefaultHexSize, Ambient: domain.AmbientBright,
	}, s.Now())
}

// List returns the Campaign's Maps. DM only.
func (s *Maps) List(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Maps(ctx, campaign)
}

// Get returns one Map. DM only.
func (s *Maps) Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.MapID) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	return s.Repo.GetMap(ctx, campaign, id)
}

// MapEdit is the Map's name, calibration and ambient light.
type MapEdit struct {
	Name    string
	HexSize float64
	OriginX float64
	OriginY float64
	Ambient string
}

// Update renames and calibrates a Map. DM only.
func (s *Maps) Update(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.MapID, e MapEdit) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	m, err := s.Repo.GetMap(ctx, campaign, id)
	if err != nil {
		return domain.Map{}, err
	}
	if m.Name, err = mapName(e.Name); err != nil {
		return domain.Map{}, err
	}
	switch {
	case e.HexSize < 8 || e.HexSize > 400:
		return domain.Map{}, apperr.Refuse("hexes are 8 to 400 pixels from centre to corner")
	case e.Ambient != domain.AmbientBright && e.Ambient != domain.AmbientDim && e.Ambient != domain.AmbientDark:
		return domain.Map{}, apperr.Refuse("ambient light is bright, dim or dark")
	}
	m.HexSize, m.OriginX, m.OriginY, m.Ambient = e.HexSize, e.OriginX, e.OriginY, e.Ambient
	if err := s.Repo.UpdateMap(ctx, m, s.Now()); err != nil {
		return domain.Map{}, err
	}
	return m, nil
}

// Image returns the Map's picture: whole for the DM, and for everyone else with every hex the party has
// never seen painted black on the server, so undiscovered areas never reach a player's device.
func (s *Maps) Image(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.MapID) (string, []byte, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return "", nil, err
	}
	board, err := s.Repo.LoadMap(ctx, campaign, id)
	if err != nil {
		return "", nil, err
	}
	data, err := s.Blobs.Get(ctx, board.Map.ImageKey)
	if err != nil {
		return "", nil, err
	}
	if me.DM {
		return board.Map.ImageType, data, nil
	}
	masked, err := imaging.Mask(data, board.Map.Layout(), board.Reveals)
	return "image/png", masked, err
}
