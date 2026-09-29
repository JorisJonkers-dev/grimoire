package crosscheck_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/crosscheck"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

func listing(indexes ...string) string {
	parts := make([]string, 0, len(indexes))
	for _, i := range indexes {
		parts = append(parts, `{"index":"`+i+`"}`)
	}
	return `{"results":[` + strings.Join(parts, ",") + `]}`
}

func routes() map[string]string {
	return map[string]string{
		"/api/2014/spells":           listing("fire-bolt", "wish"),
		"/api/2014/spells/fire-bolt": `{"level":1}`,
		"/api/2014/classes":          listing("fighter"),
		"/api/2014/races":            listing("dwarf"),
		"/api/2014/backgrounds":      listing(),
		"/api/2014/feats":            listing(),
		"/api/2014/conditions":       listing("prone"),
		"/api/2014/magic-items":      listing("bag-of-holding"),
		"/api/2014/monsters":         listing("goblin"),
		"/api/2014/monsters/goblin":  `{"challenge_rating":0.25,"xp":50,"hit_points":7,"armor_class":[{"value":15}]}`,
		"/api/2024/spells":           listing(),
		"/api/2024/classes":          listing(),
		"/api/2024/species":          listing("elf"),
		"/api/2024/backgrounds":      listing(),
		"/api/2024/feats":            listing(),
		"/api/2024/conditions":       listing(),
		"/api/2024/magic-items":      listing(),
		"/api/2024/monsters":         listing(),
	}
}

func server(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.URL.Path]
		switch {
		case !ok:
			http.NotFound(w, r)
		case body == "boom":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			_, _ = fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func snap() snapshot.Snapshot {
	e := func(slug string) snapshot.Entry { return snapshot.Entry{Document: "srd-2014", Slug: slug, Name: slug} }
	return snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2014"}, {Key: "srd-2024"}},
		Spells:    []snapshot.Spell{{Document: "srd-2014", Slug: "fire-bolt", Level: 0}, {Document: "srd-2024", Slug: "fire-bolt"}},
		Classes:   []snapshot.Class{{Entry: e("fighter")}, {Entry: e("champion"), Parent: "fighter"}},
		Species:   []snapshot.Species{{Entry: e("dwarf")}, {Entry: e("hill-dwarf"), Subspecies: true}},
		Feats:     []snapshot.Feat{{Entry: e("grappler")}},
		Conditions: []snapshot.Condition{
			{Document: "srd-2014", Slug: "prone"}, {Document: "srd-2024", Slug: "prone"},
		},
		Items:    []snapshot.Item{{Entry: e("rope")}, {Entry: e("bag-of-holding"), Magic: true}},
		Monsters: []snapshot.Monster{{Entry: e("goblin"), ChallengeRating: 0.25, XP: 50, HitPoints: 8, ArmorClass: 15}},
	}
}

func TestRunReportsPresenceAndFieldDifferences(t *testing.T) {
	t.Parallel()
	srv := server(t, routes())
	r, err := crosscheck.Client{BaseURL: srv.URL, HTTP: srv.Client(), Workers: 2}.Run(context.Background(), snap())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Compared) != 16 || len(r.Unavailable) != 0 {
		t.Fatalf("compared %v unavailable %v", r.Compared, r.Unavailable)
	}
	if len(r.Missing) != 2 || r.Missing[0].Slug != "wish" || r.Missing[1].Slug != "elf" {
		t.Fatalf("missing = %+v", r.Missing)
	}
	extras := []string{}
	for _, f := range r.Extra {
		extras = append(extras, f.Ruleset+"/"+f.Slug)
	}
	if strings.Join(extras, ",") != "srd-2014/grappler,srd-2024/fire-bolt,srd-2024/prone" {
		t.Fatalf("extra = %v", extras)
	}
	if len(r.Disagreements) != 2 || r.Disagreements[0].Field != "level" || r.Disagreements[1].Field != "hit points" ||
		r.Disagreements[1].Ours != "8" || r.Disagreements[1].Theirs != "7" {
		t.Fatalf("disagreements = %+v", r.Disagreements)
	}
	md := r.Markdown()
	for _, want := range []string{"## Only in 5e-bits (2)", "| srd-2014 | monster | goblin | hit points | 8 | 7 |", "| srd-2014 | spell | wish |"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}

func TestRunMarksMissingResourcesUnavailable(t *testing.T) {
	t.Parallel()
	all := routes()
	delete(all, "/api/2024/species")
	srv := server(t, all)
	r, err := crosscheck.Client{BaseURL: srv.URL, HTTP: srv.Client()}.Run(context.Background(), snap())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Unavailable) != 1 || r.Unavailable[0] != "srd-2024 species" || !strings.Contains(r.Markdown(), "Not offered by 5e-bits: srd-2024 species.") {
		t.Fatalf("unavailable = %v", r.Unavailable)
	}
}

func TestRunSurfacesErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"listing status": "/api/2014/spells",
		"detail status":  "/api/2014/monsters/goblin",
	}
	for name, path := range cases {
		all := routes()
		all[path] = "boom"
		srv := server(t, all)
		if _, err := (crosscheck.Client{BaseURL: srv.URL, HTTP: srv.Client()}).Run(context.Background(), snap()); err == nil {
			t.Errorf("%s ignored", name)
		}
	}
	garbled := routes()
	garbled["/api/2014/spells"] = "{"
	srv := server(t, garbled)
	if _, err := (crosscheck.Client{BaseURL: srv.URL, HTTP: srv.Client()}).Run(context.Background(), snap()); err == nil {
		t.Error("decode error ignored")
	}
	if _, err := (crosscheck.Client{BaseURL: "http://127.0.0.1:1", HTTP: http.DefaultClient}).Run(context.Background(), snap()); err == nil {
		t.Error("connection error ignored")
	}
	if _, err := (crosscheck.Client{BaseURL: "://bad", HTTP: http.DefaultClient}).Run(context.Background(), snap()); err == nil {
		t.Error("bad url ignored")
	}
}

func TestEmptyReportRendersNone(t *testing.T) {
	t.Parallel()
	md := crosscheck.Report{}.Markdown()
	if !strings.Contains(md, "Compared: nothing.") || !strings.Contains(md, "## Disagreements (0)\n\nNone.") {
		t.Fatal(md)
	}
}
