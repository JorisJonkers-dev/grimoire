package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
)

type fakeCompendium struct {
	versionErr error
	listErr    error
	getErr     error
	sourcesErr error
	spells     []compendium.SpellSummary
	lastFilter *compendium.SpellFilter
}

func (f *fakeCompendium) Version(context.Context) (int64, error) { return 7, f.versionErr }

func (f *fakeCompendium) ListSpells(_ context.Context, filter compendium.SpellFilter) ([]compendium.SpellSummary, error) {
	f.lastFilter = &filter
	if len(f.spells) > filter.PageSize {
		return f.spells[:filter.PageSize], f.listErr
	}
	return f.spells, f.listErr
}

func (f *fakeCompendium) GetSpell(_ context.Context, slug, _ string) (compendium.Spell, error) {
	feet := 150
	return compendium.Spell{
		SpellSummary: compendium.SpellSummary{Slug: slug, Name: "Fireball", Level: 3, School: "evocation", Ruleset: "srd-2024"},
		CastingTime:  "action", RangeText: "150 feet", RangeFeet: &feet, Duration: "instantaneous", Description: "Boom, prone.",
		Classes: []string{"wizard"}, DamageTypes: []string{"fire"}, SaveAbility: "dexterity", DamageRoll: "8d6",
		MaterialText: "bat guano", HigherLevel: "More.", Scaling: []compendium.Scaling{{Kind: "slot", Level: 4, DamageRoll: "9d6"}},
		Mentions: []compendium.Condition{{Slug: "prone", Name: "Prone", Description: "Down."}},
	}, f.getErr
}

func (f *fakeCompendium) ListSources(context.Context) ([]compendium.Source, error) {
	return []compendium.Source{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, License: "CC-BY-4.0", Attribution: "a", URL: "https://x"}}, f.sourcesErr
}

func compendiumServer(t *testing.T, c *fakeCompendium) http.Handler {
	t.Helper()
	h, err := httpapi.New(httpapi.Options{
		Handler:   &httpapi.Handler{Version: "1", Store: fakeStore{}, Compendium: c, Log: quiet},
		RateLimit: 100, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func getWith(h http.Handler, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.Header.Set("X-User-Id", "u")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListSpellsPagesWithCursorAndETag(t *testing.T) {
	t.Parallel()
	c := &fakeCompendium{spells: []compendium.SpellSummary{
		{Slug: "alarm", Name: "Alarm", Level: 1, School: "abjuration", Ruleset: "srd-2024"},
		{Slug: "fire-bolt", Name: "Fire Bolt", School: "evocation", Ruleset: "srd-2024"},
		{Slug: "light", Name: "Light", School: "evocation", Ruleset: "srd-2024"},
	}}
	h := compendiumServer(t, c)
	rec := getWith(h, "/api/v1/compendium/spells?limit=2&q=a&level=1&school=abjuration&class=wizard&ruleset=srd-2024", nil)
	if rec.Code != 200 || rec.Header().Get("ETag") != `"c7"` {
		t.Fatalf("list: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	body := decode(t, rec)
	items, _ := body["items"].([]any)
	next, _ := body["nextCursor"].(string)
	if len(items) != 2 || next == "" {
		t.Fatalf("page = %v", body)
	}
	f := c.lastFilter
	if f.PageSize != 3 || f.Query != "a" || *f.Level != 1 || f.School != "abjuration" || f.Class != "wizard" || f.Ruleset != "srd-2024" {
		t.Fatalf("filter = %+v", f)
	}
	rec = getWith(h, "/api/v1/compendium/spells?cursor="+next, nil)
	if rec.Code != 200 || c.lastFilter.After == nil || c.lastFilter.After.Slug != "fire-bolt" || c.lastFilter.PageSize != 51 {
		t.Fatalf("second page: %d %+v", rec.Code, c.lastFilter)
	}
	if _, ok := decode(t, rec)["nextCursor"]; ok {
		t.Fatal("last page must not have a cursor")
	}
	if rec := getWith(h, "/api/v1/compendium/spells", map[string]string{"If-None-Match": `"c7"`}); rec.Code != http.StatusNotModified {
		t.Fatalf("conditional: %d", rec.Code)
	}
	if rec := getWith(h, "/api/v1/compendium/spells?cursor=AAAA", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor: %d", rec.Code)
	}
}

func TestGetSpellMapsEveryField(t *testing.T) {
	t.Parallel()
	h := compendiumServer(t, &fakeCompendium{})
	rec := getWith(h, "/api/v1/compendium/spells/fireball", nil)
	if rec.Code != 200 {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	b := decode(t, rec)
	for k, v := range map[string]any{"name": "Fireball", "rangeFeet": float64(150), "saveAbility": "dexterity", "damageRoll": "8d6", "materialText": "bat guano", "higherLevel": "More."} {
		if b[k] != v {
			t.Errorf("%s = %v", k, b[k])
		}
	}
	if m, _ := b["mentions"].([]any); len(m) != 1 {
		t.Fatalf("mentions = %v", b["mentions"])
	}
	if rec := getWith(h, "/api/v1/compendium/spells/fireball", map[string]string{"If-None-Match": `"c7"`}); rec.Code != http.StatusNotModified {
		t.Fatalf("conditional: %d", rec.Code)
	}
}

func TestCompendiumErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		c    *fakeCompendium
		path string
		code int
	}{
		{&fakeCompendium{getErr: compendium.ErrNotFound}, "/api/v1/compendium/spells/nothing", 404},
		{&fakeCompendium{getErr: boom}, "/api/v1/compendium/spells/x", 503},
		{&fakeCompendium{versionErr: boom}, "/api/v1/compendium/spells/x", 503},
		{&fakeCompendium{versionErr: boom}, "/api/v1/compendium/spells", 503},
		{&fakeCompendium{listErr: boom}, "/api/v1/compendium/spells", 503},
		{&fakeCompendium{sourcesErr: boom}, "/api/v1/compendium/sources", 503},
	}
	for _, tc := range cases {
		if rec := getWith(compendiumServer(t, tc.c), tc.path, nil); rec.Code != tc.code {
			t.Errorf("%s: %d want %d", tc.path, rec.Code, tc.code)
		}
	}
}

func TestListSourcesCarriesAttribution(t *testing.T) {
	t.Parallel()
	rec := getWith(compendiumServer(t, &fakeCompendium{}), "/api/v1/compendium/sources", nil)
	if rec.Code != 200 || rec.Body.String() == "" {
		t.Fatalf("sources: %d", rec.Code)
	}
}
