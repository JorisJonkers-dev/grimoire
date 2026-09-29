package open5e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/open5e"
)

func server(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if r.URL.Query().Get("page") != "" {
			key += "?page=" + r.URL.Query().Get("page")
		}
		body, ok := routes[key]
		if !ok {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprint(w, strings.ReplaceAll(body, "BASE", srv.URL))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const spellsPage1 = `{"next":"BASE/v2/spells/?page=2","results":[{"key":"srd-2024_fire-bolt","document":{"key":"srd-2024"},"name":"Fire Bolt",
 "desc":" Hurl fire. ","level":0,"school":{"key":"evocation"},"classes":[{"key":"srd-2024_wizard"},{"key":"srd-2024_sorcerer"}],
 "range_text":"120 feet","range":120,"range_unit":"feet","casting_time":"action","verbal":true,"somatic":true,"material":false,
 "saving_throw_ability":"","attack_roll":true,"damage_roll":"1d10","damage_types":["fire"],"duration":"instantaneous",
 "casting_options":[{"type":"player_level_5","damage_roll":"2d10"},{"type":"default","damage_roll":null},{"type":"slot_level_x","damage_roll":"1d4"},{"type":"player_level_11","damage_roll":""}]}]}`

const spellsPage2 = `{"next":null,"results":[{"key":"srd-2014_teleport","document":{"key":"srd-2014"},"name":"Teleport","desc":"Go.",
 "level":7,"school":{"key":"conjuration"},"classes":[],"range_text":"10 miles","range":10,"range_unit":"miles","casting_time":"action",
 "saving_throw_ability":"wisdom","damage_types":[],"duration":"instantaneous","casting_options":[{"type":"slot_level_8","damage_roll":"2d6"}]}]}`

const conditions = `{"next":null,"results":[{"key":"prone","name":"Prone","descriptions":[
 {"desc":"On the ground.","document":"srd-2024"},{"desc":"Low.","document":"srd-2014"},{"desc":"Other.","document":"a5e-ag"}]}]}`

func TestFetchMapsSpellsAndConditions(t *testing.T) {
	t.Parallel()
	srv := server(t, map[string]string{"/v2/spells/": spellsPage1, "/v2/spells/?page=2": spellsPage2, "/v2/conditions/": conditions})
	snap, err := open5e.Client{BaseURL: srv.URL, HTTP: srv.Client()}.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Documents) != 2 || len(snap.Spells) != 2 || len(snap.Conditions) != 2 {
		t.Fatalf("snapshot sizes: %d %d %d", len(snap.Documents), len(snap.Spells), len(snap.Conditions))
	}
	teleport, bolt := snap.Spells[0], snap.Spells[1]
	if bolt.Slug != "fire-bolt" || bolt.Description != "Hurl fire." || *bolt.RangeFeet != 120 || len(bolt.Classes) != 2 || bolt.Classes[0] != "wizard" {
		t.Fatalf("fire bolt = %+v", bolt)
	}
	if len(bolt.Scaling) != 1 || bolt.Scaling[0].Kind != "character" || bolt.Scaling[0].Level != 5 {
		t.Fatalf("scaling = %+v", bolt.Scaling)
	}
	if teleport.RangeFeet != nil || teleport.SaveAbility != "wisdom" || teleport.Scaling[0].Kind != "slot" {
		t.Fatalf("teleport = %+v", teleport)
	}
	if snap.Conditions[0].Document != "srd-2014" || snap.Conditions[1].Description != "On the ground." {
		t.Fatalf("conditions = %+v", snap.Conditions)
	}
}

func TestFetchSurfacesErrors(t *testing.T) {
	t.Parallel()
	failing := server(t, map[string]string{})
	if _, err := (open5e.Client{BaseURL: failing.URL, HTTP: failing.Client()}).Fetch(context.Background()); err == nil {
		t.Fatal("status error ignored")
	}
	garbled := server(t, map[string]string{"/v2/spells/": "{"})
	if _, err := (open5e.Client{BaseURL: garbled.URL, HTTP: garbled.Client()}).Fetch(context.Background()); err == nil {
		t.Fatal("decode error ignored")
	}
	noConditions := server(t, map[string]string{"/v2/spells/": `{"next":null,"results":[]}`})
	if _, err := (open5e.Client{BaseURL: noConditions.URL, HTTP: noConditions.Client()}).Fetch(context.Background()); err == nil {
		t.Fatal("condition error ignored")
	}
	if _, err := (open5e.Client{BaseURL: "http://127.0.0.1:1", HTTP: http.DefaultClient}).Fetch(context.Background()); err == nil {
		t.Fatal("connection error ignored")
	}
	if _, err := (open5e.Client{BaseURL: "://bad", HTTP: http.DefaultClient}).Fetch(context.Background()); err == nil {
		t.Fatal("bad url ignored")
	}
}
