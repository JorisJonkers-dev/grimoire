package app

import (
	"context"
	"crypto/sha256"
	hexenc "encoding/hex"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/imaging"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/atlas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
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

// A new Map's grid is drawn faintly, and a cell of a world Map covers six miles.
const (
	DefaultGridStrength = 20
	DefaultScaleMiles   = 6.0
)

// Hexes run from 8 to 400 pixels from centre to corner.
const (
	minHexSize = 8.0
	maxHexSize = 400.0
)

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

// Upload stores a local or world map picture and starts it with a default calibration. DM only.
func (s *Maps) Upload(ctx context.Context, c caller.Caller, campaign uuid.UUID, name, kind string, data []byte) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	if kind != domain.MapLocal && kind != domain.MapWorld {
		return domain.Map{}, apperr.Refuse("a map is local or world")
	}
	name, err := mapName(name)
	if err != nil {
		return domain.Map{}, err
	}
	if len(data) > MaxMapBytes {
		return domain.Map{}, apperr.Refuse("map pictures must be at most 25 MB")
	}
	return s.keep(ctx, campaign, name, kind, data)
}

// keep stores a picture and the Map of it, with a default calibration.
func (s *Maps) keep(ctx context.Context, campaign uuid.UUID, name, kind string, data []byte) (domain.Map, error) {
	info, err := imaging.Inspect(data)
	if err != nil {
		return domain.Map{}, apperr.Refuse("map pictures must be PNG, JPEG or WebP of at most 36 megapixels")
	}
	sum := sha256.Sum256(data)
	key := "sha256/" + hexenc.EncodeToString(sum[:]) + "." + strings.TrimPrefix(info.ContentType, "image/")
	if err := s.Blobs.Put(ctx, key, info.ContentType, data); err != nil {
		return domain.Map{}, err
	}
	return s.Repo.InsertMap(ctx, domain.Map{
		CampaignID: campaign, Name: name, Kind: kind, ImageKey: key, ImageType: info.ContentType, Width: info.Width, Height: info.Height,
		HexSize: DefaultHexSize, OriginX: DefaultHexSize * math.Sqrt(3) / 2, OriginY: DefaultHexSize, Ambient: domain.AmbientBright,
		Grid: domain.GridHexes, GridStrength: DefaultGridStrength, ScaleMiles: DefaultScaleMiles,
	}, s.Now())
}

// UseDefaultWorld gives the Campaign the painted Default World as a world Map. DM only.
func (s *Maps) UseDefaultWorld(ctx context.Context, c caller.Caller, campaign uuid.UUID) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	return s.keep(ctx, campaign, "Default World", domain.MapWorld, atlas.DefaultWorld())
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

// MapEdit is the Map's name, calibration and ambient light, and what of its grid and scale to change:
// a Grid left empty, and a strength or scale left nil, stay as they are.
type MapEdit struct {
	Name         string
	HexSize      float64
	OriginX      float64
	OriginY      float64
	Ambient      string
	Grid         string
	GridStrength *int
	ScaleMiles   *float64
}

// grid applies the grid and scale of an edit. A local Map keeps its 5 ft hexes.
func (e MapEdit) grid(m *domain.Map) error {
	if e.Grid != "" {
		m.Grid = e.Grid
	}
	if e.GridStrength != nil {
		m.GridStrength = *e.GridStrength
	}
	if e.ScaleMiles != nil {
		m.ScaleMiles = *e.ScaleMiles
	}
	switch {
	case m.Grid != domain.GridHexes && m.Grid != domain.GridSquares && m.Grid != domain.GridOff:
		return apperr.Refuse("a grid is hexes, squares or off")
	case m.Kind != domain.MapWorld && (m.Grid != domain.GridHexes || e.ScaleMiles != nil):
		return apperr.Refuse("a battle map keeps its 5 ft hexes")
	case m.GridStrength < 0 || m.GridStrength > 100:
		return apperr.Refuse("a grid is 0 to 100 strong")
	case !(m.ScaleMiles >= 0.1 && m.ScaleMiles <= 1000):
		return apperr.Refuse("a cell covers 0.1 to 1000 miles")
	}
	return nil
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
	case e.HexSize < minHexSize || e.HexSize > maxHexSize:
		return domain.Map{}, apperr.Refuse("hexes are 8 to 400 pixels from centre to corner")
	case e.Ambient != domain.AmbientBright && e.Ambient != domain.AmbientDim && e.Ambient != domain.AmbientDark:
		return domain.Map{}, apperr.Refuse("ambient light is bright, dim or dark")
	}
	if err := e.grid(&m); err != nil {
		return domain.Map{}, err
	}
	m.HexSize, m.OriginX, m.OriginY, m.Ambient = e.HexSize, e.OriginX, e.OriginY, e.Ambient
	if err := s.Repo.UpdateMap(ctx, m, s.Now()); err != nil {
		return domain.Map{}, err
	}
	return m, nil
}

// Calibration is two points on a Map's picture and how far apart they are in the world: feet on a local
// Map, miles on a world Map.
type Calibration struct {
	A, B     hex.Point
	Distance float64
}

// Calibrate sizes a Map's grid so that two points on its picture are a known distance apart, and
// centres a cell on the first. DM only.
func (s *Maps) Calibrate(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.MapID, k Calibration) (domain.Map, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Map{}, err
	}
	m, err := s.Repo.GetMap(ctx, campaign, id)
	if err != nil {
		return domain.Map{}, err
	}
	on := func(p hex.Point) bool {
		return p.X >= 0 && p.Y >= 0 && p.X <= float64(m.Width) && p.Y <= float64(m.Height)
	}
	if !on(k.A) || !on(k.B) {
		return domain.Map{}, apperr.Refuse("pick both points on the picture")
	}
	size, ok := hex.Calibrate(k.A, k.B, k.Distance/m.CellSpan())
	switch {
	case !ok:
		return domain.Map{}, apperr.Refuse("pick two different points and say how far apart they are")
	case size < minHexSize || size > maxHexSize:
		return domain.Map{}, apperr.Refuse("that makes hexes smaller than 8 or larger than 400 pixels from centre to corner")
	}
	m.HexSize, m.OriginX, m.OriginY = size, k.A.X, k.A.Y
	if err := s.Repo.UpdateMap(ctx, m, s.Now()); err != nil {
		return domain.Map{}, err
	}
	return m, nil
}

// Image returns the Map's picture: whole for the DM, and for everyone else with every hex the party has
// never seen painted black on the server, so undiscovered areas never reach a player's device. A world
// Map the party has found is the exception: they hold the map, so they are sent all of it.
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
	// A world Map the party has found is theirs to look at, whole; the screen dims where they have not been.
	if me.DM || (board.Map.Kind == domain.MapWorld && board.Map.Found) {
		return board.Map.ImageType, data, nil
	}
	masked, err := imaging.Mask(data, board.Map.Layout(), board.Reveals)
	return "image/png", masked, err
}
