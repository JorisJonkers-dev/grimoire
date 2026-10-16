package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func postBinary(h http.Handler, path, subject string, data []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("X-User-Id", subject)
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
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
