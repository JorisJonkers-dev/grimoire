package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 16)...)
	webpBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)
)

func upload(h http.Handler, path string, data []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, bytes.NewReader(data))
	req.Header.Set("X-User-Id", "player")
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func roundTrip(t *testing.T, h http.Handler, path string, data []byte, want string) {
	t.Helper()
	if rec := upload(h, path, data); rec.Code != http.StatusNoContent {
		t.Fatalf("upload %s: %d %s", path, rec.Code, rec.Body.String())
	}
	rec := call(h, http.MethodGet, path, "dm", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != want || !strings.Contains(rec.Header().Get("Cache-Control"), "private") ||
		!bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("get %s: %d %v", path, rec.Code, rec.Header())
	}
}

func TestPicturesOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	chID, _ := decode(t, call(h, http.MethodPost, base, "player", build))["id"].(string)
	one := base + "/" + chID
	if rec := call(h, http.MethodGet, one+"/portrait", "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no portrait yet: %d", rec.Code)
	}
	steps := []struct {
		path string
		data []byte
		want string
	}{
		{"/portrait", pngBytes, "image/png"},
		{"/token", jpegBytes, "image/jpeg"},
		{"/token", webpBytes, "image/webp"},
		{"/portrait", webpBytes, "image/webp"},
		{"/portrait", jpegBytes, "image/jpeg"},
		{"/token", pngBytes, "image/png"},
	}
	for _, st := range steps {
		roundTrip(t, h, one+st.path, st.data, st.want)
	}
	sheet := decode(t, call(h, http.MethodGet, one, "player", ""))
	portrait, _ := sheet["portraitUrl"].(string)
	if !strings.HasPrefix(portrait, one+"/portrait?v=") || sheet["tokenUrl"] == nil {
		t.Fatalf("urls = %v %v", sheet["portraitUrl"], sheet["tokenUrl"])
	}
	if !strings.Contains(call(h, http.MethodGet, base, "player", "").Body.String(), `"tokenUrl":"/api/v1/`) {
		t.Fatal("party list lacks the token url")
	}
	if rec := call(h, http.MethodDelete, one+"/token", "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("clear: %d", rec.Code)
	}
	if rec := upload(h, one+"/portrait", []byte("GIF89a")); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("gif: %d", rec.Code)
	}
	if rec := upload(h, one+"/token", make([]byte, app.MaxImageBytes+1)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("huge: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, one+"/portrait", "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger: %d", rec.Code)
	}
}
