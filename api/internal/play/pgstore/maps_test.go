package pgstore_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

func mapPicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			img.Set(x, y, color.RGBA{R: 180, G: 160, B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func maps(tb table, repo app.MapRepository, blobs app.Blobs) *app.Maps {
	return &app.Maps{Repo: repo, Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Blobs: blobs, Now: time.Now}
}

func TestMapsUploadCalibrateAndMaskTheirPicture(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s := maps(tb, pgstore.New(tb.pool), storage.Dir{Path: t.TempDir()})
	m, err := s.Upload(ctx, dm, tb.campaign, " Crypt ", "local", mapPicture(t))
	if err != nil || m.Name != "Crypt" || m.Width != 400 || m.ImageType != "image/png" || m.HexSize != app.DefaultHexSize {
		t.Fatalf("upload = %+v %v", m, err)
	}
	m, err = s.Update(ctx, dm, tb.campaign, m.ID, app.MapEdit{Name: "Crypt level 1", HexSize: 40, OriginX: 34.64, OriginY: 40, Ambient: domain.AmbientDark})
	if err != nil || m.Ambient != domain.AmbientDark || m.OriginX != 34.64 {
		t.Fatalf("calibrate = %+v %v", m, err)
	}
	if list, err := s.List(ctx, dm, tb.campaign); err != nil || len(list) != 1 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if got, err := s.Get(ctx, dm, tb.campaign, m.ID); err != nil || got.Name != "Crypt level 1" {
		t.Fatalf("get = %+v %v", got, err)
	}
	contentType, full, err := s.Image(ctx, dm, tb.campaign, m.ID)
	if err != nil || contentType != "image/png" || !bytes.Equal(full, mapPicture(t)) {
		t.Fatalf("dm picture = %s %v", contentType, err)
	}
	if err := pgstore.New(tb.pool).AddRevealForTest(ctx, m.ID, hex.Coord{Q: 0, R: 0}); err != nil {
		t.Fatal(err)
	}
	contentType, masked, err := s.Image(ctx, player, tb.campaign, m.ID)
	if err != nil || contentType != "image/png" {
		t.Fatalf("player picture = %s %v", contentType, err)
	}
	img, err := png.Decode(bytes.NewReader(masked))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(34, 40).RGBA(); r == 0 {
		t.Fatal("the remembered hex went black")
	}
	unseen := m.Layout().ToPixel(hex.Coord{Q: 3, R: 1})
	if r, g, b, _ := img.At(int(unseen.X), int(unseen.Y)).RGBA(); r != 0 || g != 0 || b != 0 {
		t.Fatal("a never-seen hex reached the player's picture")
	}
}

func TestMapsRefuseAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s := maps(tb, pgstore.New(tb.pool), storage.Dir{Path: t.TempDir()})
	m, _ := s.Upload(ctx, dm, tb.campaign, "Crypt", "local", mapPicture(t))
	var rule *apperr.RuleError
	bad := map[string]error{}
	_, bad["name"] = s.Upload(ctx, dm, tb.campaign, " ", "local", mapPicture(t))
	_, bad["huge"] = s.Upload(ctx, dm, tb.campaign, "Big", "local", make([]byte, app.MaxMapBytes+1))
	_, bad["garbage"] = s.Upload(ctx, dm, tb.campaign, "Junk", "local", []byte("not a picture"))
	_, bad["kind"] = s.Upload(ctx, dm, tb.campaign, "Odd", "dungeon", mapPicture(t))
	_, bad["hex"] = s.Update(ctx, dm, tb.campaign, m.ID, app.MapEdit{Name: "A", HexSize: 2, Ambient: domain.AmbientDim})
	_, bad["ambient"] = s.Update(ctx, dm, tb.campaign, m.ID, app.MapEdit{Name: "A", HexSize: 40, Ambient: "noon"})
	_, bad["rename"] = s.Update(ctx, dm, tb.campaign, m.ID, app.MapEdit{Name: " ", HexSize: 40, Ambient: domain.AmbientDim})
	for name, err := range bad {
		if !errors.As(err, &rule) {
			t.Errorf("%s: %v", name, err)
		}
	}
	forbidden := map[string]error{}
	_, forbidden["upload"] = s.Upload(ctx, player, tb.campaign, "Mine", "local", mapPicture(t))
	_, forbidden["list"] = s.List(ctx, player, tb.campaign)
	_, forbidden["get"] = s.Get(ctx, player, tb.campaign, m.ID)
	_, forbidden["update"] = s.Update(ctx, player, tb.campaign, m.ID, app.MapEdit{Name: "A", HexSize: 40, Ambient: domain.AmbientDim})
	for name, err := range forbidden {
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("player %s: %v", name, err)
		}
	}
	missing := map[string]error{}
	_, missing["stranger"] = s.List(ctx, stranger, tb.campaign)
	_, _, missing["stranger image"] = s.Image(ctx, stranger, tb.campaign, m.ID)
	_, _, missing["image"] = s.Image(ctx, dm, tb.campaign, domain.MapID(uuid.New()))
	_, missing["update"] = s.Update(ctx, dm, tb.campaign, domain.MapID(uuid.New()), app.MapEdit{Name: "A", HexSize: 40, Ambient: domain.AmbientDim})
	for name, err := range missing {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := pgstore.New(tb.pool).UpdateMap(ctx, domain.Map{CampaignID: tb.campaign, ID: domain.MapID(uuid.New())}, time.Now()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("update missing = %v", err)
	}
	broken := maps(tb, pgstore.New(tb.pool), brokenBlobs{})
	if _, err := broken.Upload(ctx, dm, tb.campaign, "A", "local", mapPicture(t)); err == nil {
		t.Fatal("storage failure on upload ignored")
	}
	if _, _, err := broken.Image(ctx, dm, tb.campaign, m.ID); err == nil {
		t.Fatal("storage failure on read ignored")
	}
}

// A world Map has a scale and a grid the DM chooses; two points a known distance apart size the grid.
func TestMapsScaleAndCalibrate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s := maps(tb, pgstore.New(tb.pool), storage.Dir{Path: t.TempDir()})
	world, err := s.Upload(ctx, dm, tb.campaign, "Realm", "world", mapPicture(t))
	if err != nil || world.Grid != domain.GridHexes || world.GridStrength != app.DefaultGridStrength || world.ScaleMiles != app.DefaultScaleMiles {
		t.Fatalf("a new world map = %+v %v", world, err)
	}
	strength, miles := 45, 12.0
	edit := app.MapEdit{Name: "Realm", HexSize: 40, Ambient: domain.AmbientBright, Grid: domain.GridSquares, GridStrength: &strength, ScaleMiles: &miles}
	if _, err := s.Update(ctx, dm, tb.campaign, world.ID, edit); err != nil {
		t.Fatal(err)
	}
	// Left out of an edit, the grid and scale stay as they were.
	if _, err := s.Update(ctx, dm, tb.campaign, world.ID, app.MapEdit{Name: "The Realm", HexSize: 40, Ambient: domain.AmbientBright}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, dm, tb.campaign, world.ID)
	if err != nil || got.Name != "The Realm" || got.Grid != domain.GridSquares || got.GridStrength != 45 || got.ScaleMiles != 12 {
		t.Fatalf("the edited world map = %+v %v", got, err)
	}
	// Ten cells of 12 miles lie between two points 120 miles apart: the first point becomes a cell's centre.
	a := hex.Point{X: 20, Y: 30}
	b := hex.Point{X: 20 + 10*hex.Across(20), Y: 30}
	if _, err := s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: b, Distance: 120}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, dm, tb.campaign, world.ID)
	if err != nil || math.Abs(got.HexSize-20) > 1e-9 || got.OriginX != 20 || got.OriginY != 30 || got.Grid != domain.GridSquares {
		t.Fatalf("the calibrated world map = %+v %v", got, err)
	}
	// A battle map keeps its 5 ft hexes: 50 feet are ten of them, and it takes no other grid or scale.
	local, _ := s.Upload(ctx, dm, tb.campaign, "Crypt", "local", mapPicture(t))
	if local.Grid != domain.GridHexes {
		t.Fatalf("a new battle map = %+v", local)
	}
	if got, err := s.Calibrate(ctx, dm, tb.campaign, local.ID, app.Calibration{A: a, B: b, Distance: 50}); err != nil || math.Abs(got.HexSize-20) > 1e-9 {
		t.Fatalf("the calibrated battle map = %+v %v", got, err)
	}
	none, hexes := 0, app.MapEdit{Name: "Crypt", HexSize: 40, Ambient: domain.AmbientDim, Grid: domain.GridHexes}
	hexes.GridStrength = &none
	if got, err := s.Update(ctx, dm, tb.campaign, local.ID, hexes); err != nil || got.GridStrength != 0 || got.Grid != domain.GridHexes {
		t.Fatalf("a battle map with a faint grid = %+v %v", got, err)
	}
	ok := app.MapEdit{Name: "A", HexSize: 40, Ambient: domain.AmbientDim}
	with := func(change func(*app.MapEdit)) app.MapEdit { e := ok; change(&e); return e }
	number := func(v float64) *float64 { return &v }
	whole := func(v int) *int { return &v }
	var rule *apperr.RuleError
	bad := map[string]error{}
	_, bad["an unknown grid"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.Grid = "triangles" }))
	_, bad["a grid below nothing"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.GridStrength = whole(-1) }))
	_, bad["a grid past solid"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.GridStrength = whole(101) }))
	_, bad["cells of no miles"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.ScaleMiles = number(0) }))
	_, bad["cells of too many miles"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.ScaleMiles = number(1000.5) }))
	_, bad["cells of no number of miles"] = s.Update(ctx, dm, tb.campaign, world.ID, with(func(e *app.MapEdit) { e.ScaleMiles = number(math.NaN()) }))
	_, bad["squares on a battle map"] = s.Update(ctx, dm, tb.campaign, local.ID, with(func(e *app.MapEdit) { e.Grid = domain.GridSquares }))
	_, bad["no grid on a battle map"] = s.Update(ctx, dm, tb.campaign, local.ID, with(func(e *app.MapEdit) { e.Grid = domain.GridOff }))
	_, bad["miles on a battle map"] = s.Update(ctx, dm, tb.campaign, local.ID, with(func(e *app.MapEdit) { e.ScaleMiles = number(6) }))
	_, bad["one point twice"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: a, Distance: 120})
	_, bad["no distance"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: b, Distance: 0})
	_, bad["a point left of the picture"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: hex.Point{X: -1, Y: 30}, B: b, Distance: 120})
	_, bad["a point above the picture"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: hex.Point{X: 20, Y: -1}, B: b, Distance: 120})
	_, bad["a point right of the picture"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: hex.Point{X: 400.5, Y: 30}, Distance: 120})
	_, bad["a point below the picture"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: hex.Point{X: 20, Y: 300.5}, Distance: 120})
	_, bad["cells too small to see"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: a, B: hex.Point{X: 20 + 10*hex.Across(7.9), Y: 30}, Distance: 120})
	_, bad["cells too large to use"] = s.Calibrate(ctx, dm, tb.campaign, world.ID, app.Calibration{A: hex.Point{X: 0, Y: 0}, B: hex.Point{X: 400, Y: 300}, Distance: 0.5})
	for name, err := range bad {
		if !errors.As(err, &rule) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// The edges of what is allowed are allowed.
	for name, c := range map[string]app.Calibration{
		"the smallest cells":       {A: a, B: hex.Point{X: 20 + 10*hex.Across(8), Y: 30}, Distance: 120},
		"the corners of a picture": {A: hex.Point{X: 0, Y: 0}, B: hex.Point{X: 400, Y: 300}, Distance: 120},
	} {
		if _, err := s.Calibrate(ctx, dm, tb.campaign, world.ID, c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, e := range map[string]app.MapEdit{
		"a solid grid":        with(func(e *app.MapEdit) { e.GridStrength = whole(100) }),
		"a thousand miles":    with(func(e *app.MapEdit) { e.ScaleMiles = number(1000) }),
		"a tenth of a mile":   with(func(e *app.MapEdit) { e.ScaleMiles = number(0.1) }),
		"no grid on the land": with(func(e *app.MapEdit) { e.Grid = domain.GridOff }),
	} {
		if _, err := s.Update(ctx, dm, tb.campaign, world.ID, e); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.Calibrate(ctx, player, tb.campaign, world.ID, app.Calibration{A: a, B: b, Distance: 120}); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("a player calibrating: %v", err)
	}
	if _, err := s.Calibrate(ctx, dm, tb.campaign, domain.MapID(uuid.New()), app.Calibration{A: a, B: b, Distance: 120}); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("calibrating no map: %v", err)
	}
}

// A player is sent a world Map whole once the party has found it, and never a battle map.
func TestAFoundWorldMapIsSentWhole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s := maps(tb, pgstore.New(tb.pool), storage.Dir{Path: t.TempDir()})
	world, _ := s.Upload(ctx, dm, tb.campaign, "Realm", "world", mapPicture(t))
	local, _ := s.Upload(ctx, dm, tb.campaign, "Crypt", "local", mapPicture(t))
	dark := func(id domain.MapID) bool {
		t.Helper()
		_, data, err := s.Image(ctx, player, tb.campaign, id)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := img.At(300, 200).RGBA()
		return r == 0 && g == 0 && b == 0
	}
	if world.Found || !dark(world.ID) || !dark(local.ID) {
		t.Fatalf("before anything is found: %+v", world)
	}
	if _, err := tb.pool.Exec(ctx, "UPDATE campaign.maps SET found = true WHERE campaign_id = $1", tb.campaign); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, dm, tb.campaign, world.ID); err != nil || !got.Found {
		t.Fatalf("the found world map = %+v %v", got, err)
	}
	if contentType, data, err := s.Image(ctx, player, tb.campaign, world.ID); err != nil || contentType != "image/png" || !bytes.Equal(data, mapPicture(t)) {
		t.Fatalf("a found world map's picture = %s %v", contentType, err)
	}
	// A found battle map keeps its Fog: what the party has not seen there stays black.
	if !dark(local.ID) {
		t.Fatal("a found battle map was sent whole")
	}
	// An edit leaves a Map found.
	if got, err := s.Update(ctx, dm, tb.campaign, world.ID, app.MapEdit{Name: "Realm", HexSize: 40, Ambient: domain.AmbientBright}); err != nil || !got.Found {
		t.Fatalf("an edited found map = %+v %v", got, err)
	}
}

// The Default World comes painted: a DM takes it as a world Map without uploading anything.
func TestTheDefaultWorldIsAWorldMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	blobs := storage.Dir{Path: t.TempDir()}
	s := maps(tb, pgstore.New(tb.pool), blobs)
	m, err := s.UseDefaultWorld(ctx, dm, tb.campaign)
	if err != nil || m.Name != "Default World" || m.Kind != domain.MapWorld || m.ImageType != "image/png" || m.Grid != domain.GridHexes || m.ScaleMiles != app.DefaultScaleMiles {
		t.Fatalf("the Default World = %+v %v", m, err)
	}
	contentType, data, err := s.Image(ctx, dm, tb.campaign, m.ID)
	if err != nil || contentType != "image/png" {
		t.Fatalf("its picture = %s %v", contentType, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != m.Width || img.Bounds().Dy() != m.Height || m.Width < 1200 || m.Height < 800 {
		t.Fatalf("its picture is %v, the Map %dx%d: %v", img.Bounds(), m.Width, m.Height, err)
	}
	// It is land in a sea: the corners are water, and the picture has several kinds of terrain.
	sea := img.At(0, 0)
	seen := map[color.Color]int{}
	for y := 0; y < m.Height; y += 4 {
		for x := 0; x < m.Width; x += 4 {
			seen[img.At(x, y)]++
		}
	}
	for _, corner := range [][2]int{{m.Width - 1, 0}, {0, m.Height - 1}, {m.Width - 1, m.Height - 1}} {
		if img.At(corner[0], corner[1]) != sea {
			t.Errorf("corner %v is not sea", corner)
		}
	}
	total := (m.Width / 4) * (m.Height / 4)
	if len(seen) < 6 || seen[sea] < total/4 || seen[sea] > total*3/4 {
		t.Errorf("%d kinds of terrain, %d of %d samples sea", len(seen), seen[sea], total)
	}
	// Taken twice it is the same picture, kept once.
	again, err := s.UseDefaultWorld(ctx, dm, tb.campaign)
	if err != nil || again.ID == m.ID || again.ImageKey != m.ImageKey {
		t.Fatalf("the Default World again = %+v %v", again, err)
	}
	if _, err := s.UseDefaultWorld(ctx, player, tb.campaign); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("a player taking the Default World: %v", err)
	}
	if _, err := maps(tb, pgstore.New(tb.pool), brokenBlobs{}).UseDefaultWorld(ctx, dm, tb.campaign); err == nil {
		t.Error("storage failure on the Default World ignored")
	}
}

type brokenBlobs struct{}

func (brokenBlobs) Put(context.Context, string, string, []byte) error { return errors.New("gone") }
func (brokenBlobs) Get(context.Context, string) ([]byte, error)       { return nil, errors.New("gone") }

func TestEveryMapDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	blobs := storage.Dir{Path: t.TempDir()}
	m, _ := maps(tb, pgstore.New(tb.pool), blobs).Upload(ctx, dm, tb.campaign, "Crypt", "local", mapPicture(t))
	ops := map[string]func(s *app.Maps) error{
		"upload": func(s *app.Maps) error {
			_, err := s.Upload(ctx, dm, tb.campaign, "Other", "local", mapPicture(t))
			return err
		},
		"list": func(s *app.Maps) error { _, err := s.List(ctx, dm, tb.campaign); return err },
		"update": func(s *app.Maps) error {
			_, err := s.Update(ctx, dm, tb.campaign, m.ID, app.MapEdit{Name: "B", HexSize: 40, Ambient: domain.AmbientDim})
			return err
		},
		"image": func(s *app.Maps) error { _, _, err := s.Image(ctx, player, tb.campaign, m.ID); return err },
		"calibrate": func(s *app.Maps) error {
			_, err := s.Calibrate(ctx, dm, tb.campaign, m.ID, app.Calibration{A: hex.Point{X: 10, Y: 10}, B: hex.Point{X: 210, Y: 10}, Distance: 25})
			return err
		},
		"default world": func(s *app.Maps) error { _, err := s.UseDefaultWorld(ctx, dm, tb.campaign); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(maps(tb, pgstore.NewFaulty(tb.pool, f), blobs))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
