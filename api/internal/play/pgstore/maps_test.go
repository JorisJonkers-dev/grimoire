package pgstore_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
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
