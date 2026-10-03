package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func mapPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for y := range 120 {
		for x := range 200 {
			img.Set(x, y, color.RGBA{R: 120, G: 200, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func postBinary(h http.Handler, path, subject string, data []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("X-User-Id", subject)
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMapsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/maps"
	if rec := postBinary(h, base+"?name=Mine", "player", mapPNG(t)); rec.Code != http.StatusForbidden {
		t.Fatalf("player upload: %d", rec.Code)
	}
	rec := postBinary(h, base+"?name=Crypt", "dm", mapPNG(t))
	m := decode(t, rec)
	mapID, _ := m["id"].(string)
	if rec.Code != http.StatusCreated || m["width"] != float64(200) || !strings.HasSuffix(m["imageUrl"].(string), "/image") { //nolint:forcetypeassert // test data
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postBinary(h, base+"?name=Junk", "dm", []byte("nope")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("junk: %d", rec.Code)
	}
	one := base + "/" + mapID
	edit := `{"name":"Crypt 1","hexSizePx":30,"originX":26,"originY":30,"ambient":"dark"}`
	if rec := call(h, http.MethodPut, one, "dm", edit); rec.Code != 200 || decode(t, rec)["ambient"] != "dark" {
		t.Fatalf("calibrate: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, one, "dm", ""); rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base, "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Crypt 1") {
		t.Fatalf("list: %d", rec.Code)
	}
	dmImg := call(h, http.MethodGet, one+"/image", "dm", "")
	if dmImg.Code != 200 || dmImg.Header().Get("Content-Type") != "image/png" || !bytes.Equal(dmImg.Body.Bytes(), mapPNG(t)) {
		t.Fatalf("dm image: %d", dmImg.Code)
	}
	playerImg := call(h, http.MethodGet, one+"/image?v=0", "player", "")
	pic, err := png.Decode(bytes.NewReader(playerImg.Body.Bytes()))
	if playerImg.Code != 200 || err != nil {
		t.Fatalf("player image: %d %v", playerImg.Code, err)
	}
	for _, pt := range [][2]int{{26, 30}, {100, 60}, {190, 110}} {
		if r, g, b, _ := pic.At(pt[0], pt[1]).RGBA(); r != 0 || g != 0 || b != 0 {
			t.Fatalf("unexplored pixel %v sent to a player", pt)
		}
	}
	if rec := call(h, http.MethodGet, one+"/image", "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger image: %d", rec.Code)
	}
	// A battle map says what its grid is, and keeps its 5 ft hexes.
	if m["gridKind"] != "hexes" || m["gridStrength"] != float64(20) || m["scaleMiles"] != float64(6) {
		t.Fatalf("a new map's grid: %s", rec.Body.String())
	}
	squares := `{"name":"Crypt 1","hexSizePx":30,"originX":26,"originY":30,"ambient":"dark","gridKind":"squares"}`
	if rec := call(h, http.MethodPut, one, "dm", squares); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("squares on a battle map: %d %s", rec.Code, rec.Body.String())
	}
	// The Default World is a world Map nobody has to upload; its grid, scale and strength are the DM's to set.
	world := "/api/v1/campaigns/" + id + "/default-world"
	if rec := call(h, http.MethodPost, world, "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player takes the Default World: %d", rec.Code)
	}
	rec = call(h, http.MethodPost, world, "dm", "")
	w := decode(t, rec)
	worldID, _ := w["id"].(string)
	if rec.Code != http.StatusCreated || w["name"] != "Default World" || w["kind"] != "world" || w["width"] != float64(1600) {
		t.Fatalf("the Default World: %d %s", rec.Code, rec.Body.String())
	}
	// A found world map's picture has a version past a thousand million, and is served at it.
	if w["found"] != false {
		t.Fatalf("a new world map is found: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/"+worldID+"/image?v=1000000007", "dm", ""); rec.Code != 200 {
		t.Fatalf("the picture at a found map's version: %d %s", rec.Code, rec.Body.String())
	}
	edit = `{"name":"The Realm","hexSizePx":24,"originX":20,"originY":24,"ambient":"bright","gridKind":"squares","gridStrength":45,"scaleMiles":12}`
	rec = call(h, http.MethodPut, base+"/"+worldID, "dm", edit)
	if w = decode(t, rec); rec.Code != 200 || w["gridKind"] != "squares" || w["gridStrength"] != float64(45) || w["scaleMiles"] != float64(12) {
		t.Fatalf("the world's grid: %d %s", rec.Code, rec.Body.String())
	}
	// Two points 120 miles apart are ten cells of 12 miles apart.
	points := `{"ax":100,"ay":200,"bx":100,"by":546.4101615137754,"distance":120}`
	if rec := call(h, http.MethodPost, base+"/"+worldID+"/calibration", "player", points); rec.Code != http.StatusForbidden {
		t.Fatalf("player calibrates: %d", rec.Code)
	}
	rec = call(h, http.MethodPost, base+"/"+worldID+"/calibration", "dm", points)
	w = decode(t, rec)
	size, _ := w["hexSizePx"].(float64)
	if rec.Code != 200 || size < 19.999 || size > 20.001 || w["originX"] != float64(100) || w["originY"] != float64(200) || w["gridKind"] != "squares" {
		t.Fatalf("calibrated: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/"+worldID+"/calibration", "dm", `{"ax":100,"ay":200,"bx":100,"by":200,"distance":120}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("one point twice: %d", rec.Code)
	}
}

type brokenMaps struct{ err error }

func (b brokenMaps) Upload(context.Context, caller.Caller, uuid.UUID, string, string, []byte) (playdomain.Map, error) {
	return playdomain.Map{}, b.err
}

func (b brokenMaps) List(context.Context, caller.Caller, uuid.UUID) ([]playdomain.Map, error) {
	return nil, b.err
}

func (b brokenMaps) Get(context.Context, caller.Caller, uuid.UUID, playdomain.MapID) (playdomain.Map, error) {
	return playdomain.Map{}, b.err
}

func (b brokenMaps) Update(context.Context, caller.Caller, uuid.UUID, playdomain.MapID, playapp.MapEdit) (playdomain.Map, error) {
	return playdomain.Map{}, b.err
}

func (b brokenMaps) Calibrate(context.Context, caller.Caller, uuid.UUID, playdomain.MapID, playapp.Calibration) (playdomain.Map, error) {
	return playdomain.Map{}, b.err
}

func (b brokenMaps) UseDefaultWorld(context.Context, caller.Caller, uuid.UUID) (playdomain.Map, error) {
	return playdomain.Map{}, b.err
}

func (b brokenMaps) Image(context.Context, caller.Caller, uuid.UUID, playdomain.MapID) (string, []byte, error) {
	if b.err == nil {
		return "image/jpeg", []byte("jpeg"), nil
	}
	return "", nil, b.err
}

type webpMaps struct{ brokenMaps }

func (webpMaps) Image(context.Context, caller.Caller, uuid.UUID, playdomain.MapID) (string, []byte, error) {
	return "image/webp", []byte("webp"), nil
}

func TestMapErrorsAndPictureTypes(t *testing.T) {
	t.Parallel()
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/maps"
	one := c + "/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.MapService(brokenMaps{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, c, ""},
		{http.MethodGet, one, ""},
		{http.MethodGet, one + "/image", ""},
		{http.MethodPut, one, `{"name":"A","hexSizePx":40,"originX":0,"originY":0,"ambient":"dim"}`},
		{http.MethodPost, one + "/calibration", `{"ax":0,"ay":0,"bx":10,"by":0,"distance":5}`},
		{http.MethodPost, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/default-world", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	if rec := postBinary(h, c+"?name=A", "u", []byte("x")); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("upload: %d", rec.Code)
	}
	for want, svc := range map[string]httpapi.MapService{"image/jpeg": brokenMaps{}, "image/webp": webpMaps{}} {
		rec := call(campaignServer(t, brokenCampaigns{}, svc), http.MethodGet, one+"/image", "u", "")
		if rec.Code != 200 || rec.Header().Get("Content-Type") != want {
			t.Errorf("%s: %d %s", want, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListMaps(ctx, oas.ListMapsParams{}))
	add(hh.UploadMap(ctx, oas.UploadMapReq{}, oas.UploadMapParams{}))
	add(hh.GetMap(ctx, oas.GetMapParams{}))
	add(hh.UpdateMap(ctx, &oas.MapEdit{}, oas.UpdateMapParams{}))
	add(hh.GetMapImage(ctx, oas.GetMapImageParams{}))
	add(hh.CalibrateMap(ctx, &oas.MapCalibration{}, oas.CalibrateMapParams{}))
	add(hh.UseDefaultWorld(ctx, oas.UseDefaultWorldParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
