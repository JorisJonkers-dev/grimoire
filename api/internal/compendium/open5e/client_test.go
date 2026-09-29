package open5e_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/open5e"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
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

const empty = `{"next":null,"results":[]}`

const classes = `{"next":null,"results":[
 {"key":"srd-2024_barbarian","document":{"key":"srd-2024"},"name":"Barbarian","desc":" Rage. ","hit_dice":"D12","caster_type":"NONE",
  "saving_throws":[{"name":"Strength"},{"name":"Constitution"}],
  "features":[{"key":"srd-2024_barbarian_rage","name":"Rage","desc":"Angry.","gained_at":[{"level":1}]},{"key":"odd","name":"Odd","desc":"","gained_at":[]}]},
 {"key":"srd-2024_berserker","document":{"key":"srd-2024"},"name":"Berserker","desc":"","hit_dice":"","caster_type":"",
  "subclass_of":{"key":"srd-2024_barbarian"},"saving_throws":[],"features":[]},
 {"key":"a5e_other","document":{"key":"a5e"},"name":"Other","desc":"","features":[]}]}`

const species = `{"next":null,"results":[{"key":"srd-2014_dwarf","document":{"key":"srd-2014"},"name":"Dwarf","desc":"Stout.",
 "is_subspecies":false,"traits":[{"name":"Darkvision","desc":" See. "}]},{"key":"x_elf","document":{"key":"x"},"name":"Elf"}]}`

const backgrounds = `{"next":null,"results":[{"key":"srd-2024_sage","document":{"key":"srd-2024"},"name":"Sage","desc":"",
 "benefits":[{"name":"Skill Proficiencies","desc":"Arcana"}]},{"key":"x_b","document":{"key":"x"},"name":"B"}]}`

const feats = `{"next":null,"results":[{"key":"srd-2024_alert","document":{"key":"srd-2024"},"name":"Alert","desc":"","type":"Origin",
 "prerequisite":"","benefits":[{"desc":" Initiative. "}]},{"key":"x_f","document":{"key":"x"},"name":"F"}]}`

const weapons = `{"next":null,"results":[{"key":"srd-2024_longbow","document":{"key":"srd-2024"},"name":"Longbow","damage_dice":"1d8",
 "damage_type":{"key":"piercing"},"range":150,"long_range":600,"is_simple":false,
 "properties":[{"property":{"name":"Ammunition","type":null},"detail":"arrow"},{"property":{"name":"Slow","type":"Mastery"},"detail":null}]},
 {"key":"x_w","document":{"key":"x"},"name":"W"}]}`

const armor = `{"next":null,"results":[{"key":"srd-2024_plate-armor","document":{"key":"srd-2024"},"name":"Plate Armor","category":"heavy",
 "ac_base":18,"ac_add_dexmod":false,"ac_cap_dexmod":null,"grants_stealth_disadvantage":true,"strength_score_required":15},
 {"key":"x_a","document":{"key":"x"},"name":"A"}]}`

const items = `{"next":null,"results":[{"key":"srd-2024_rope","document":{"key":"srd-2024"},"name":"Rope","desc":"","category":{"key":"adventuring-gear"},
 "cost":"1.00","weight":"5.000"},{"key":"srd-2024_odd","document":{"key":"srd-2024"},"name":"Odd","cost":"n/a","weight":"NaN"},
 {"key":"x_i","document":{"key":"x"},"name":"I"}]}`

const magicItems = `{"next":null,"results":[{"key":"srd-2024_bag","document":{"key":"srd-2024"},"name":"Bag","desc":"Holds.",
 "category":{"key":"wondrous-item"},"cost":"0","weight":"15","rarity":{"key":"uncommon"},"requires_attunement":true,
 "attunement_detail":"by a wizard"},{"key":"vom_x","document":{"key":"vom"},"name":"X"}]}`

const creatures = `{"next":null,"results":[{"key":"srd-2024_goblin","document":{"key":"srd-2024"},"name":"Goblin","size":{"key":"small"},
 "type":{"key":"humanoid"},"alignment":"chaotic neutral","armor_class":15,"armor_detail":"leather","hit_points":7,"hit_dice":"2d6",
 "challenge_rating":0.25,"experience_points":50,"ability_scores":{"strength":8,"dexterity":14},
 "saving_throws":{"dexterity":4,"wisdom":null},"skill_bonuses":{"sleight_of_hand":6,"stealth":null},
 "speed_all":{"walk":30,"fly":0,"hover":false},"passive_perception":9,"darkvision_range":60,"blindsight_range":null,"truesight_range":0,
 "languages":{"as_string":"Common, Goblin"},
 "resistances_and_immunities":{"damage_immunities":[],"damage_resistances":[{"key":"fire"}],"damage_vulnerabilities":[],"condition_immunities":[{"key":"charmed"}]},
 "traits":[{"name":"Nimble","desc":"Escape."}],
 "actions":[{"name":"Scimitar","desc":"Slash.","action_type":"ACTION","attacks":[{"name":"Scimitar","attack_type":"WEAPON","to_hit_mod":4,
   "reach":5,"range":null,"long_range":null,"damage_die_count":1,"damage_die_type":"D6","damage_bonus":2,"damage_type":{"key":"slashing"},
   "extra_damage_die_count":1,"extra_damage_die_type":"D4","extra_damage_type":{"key":"poison"}}]},
  {"name":"Spit","desc":"","action_type":"BONUS_ACTION","attacks":[{"name":"Spit","attack_type":"SPELL","to_hit_mod":3,
   "range":30,"long_range":60,"damage_die_count":0,"extra_damage_type":{"key":"acid"}}]}]},
 {"key":"x_c","document":{"key":"x"},"name":"C"}]}`

func allRoutes() map[string]string {
	return map[string]string{
		"/v2/spells/": spellsPage1, "/v2/spells/?page=2": spellsPage2, "/v2/conditions/": conditions,
		"/v2/classes/": classes, "/v2/species/": species, "/v2/backgrounds/": backgrounds, "/v2/feats/": feats,
		"/v2/weapons/": weapons, "/v2/armor/": armor, "/v2/items/": items, "/v2/magicitems/": magicItems, "/v2/creatures/": creatures,
	}
}

func TestFetchMapsSpellsAndConditions(t *testing.T) {
	t.Parallel()
	srv := server(t, allRoutes())
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

func TestFetchMapsEntriesPerDocument(t *testing.T) {
	t.Parallel()
	srv := server(t, allRoutes())
	snap, err := open5e.Client{BaseURL: srv.URL, HTTP: srv.Client()}.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Classes) != 2 || len(snap.Species) != 1 || len(snap.Backgrounds) != 1 || len(snap.Feats) != 1 ||
		len(snap.Weapons) != 1 || len(snap.Armor) != 1 || len(snap.Items) != 3 || len(snap.Monsters) != 1 {
		t.Fatalf("entry sizes: %+v", snap)
	}
	barbarian, berserker := snap.Classes[0], snap.Classes[1]
	if barbarian.HitDie != 12 || barbarian.CasterType != "none" || barbarian.Description != "Rage." ||
		barbarian.SavingThrows[1] != "constitution" || barbarian.Features[0].Slug != "rage" || barbarian.Features[0].Levels[0] != 1 ||
		barbarian.Features[1].Slug != "odd" {
		t.Fatalf("barbarian = %+v", barbarian)
	}
	if berserker.Parent != "barbarian" || berserker.HitDie != 0 {
		t.Fatalf("berserker = %+v", berserker)
	}
	if snap.Species[0].Traits[0].Description != "See." || snap.Backgrounds[0].Benefits[0].Name != "Skill Proficiencies" ||
		snap.Feats[0].Benefits[0] != "Initiative." || snap.Feats[0].Type != "Origin" {
		t.Fatalf("species/background/feat = %+v %+v %+v", snap.Species, snap.Backgrounds, snap.Feats)
	}
	assertWeapons(t, snap)
}

func assertWeapons(t *testing.T, snap snapshot.Snapshot) {
	t.Helper()
	bow := snap.Weapons[0]
	if bow.RangeFeet != 150 || bow.LongRangeFeet != 600 || bow.DamageType != "piercing" || bow.Properties[0].Detail != "arrow" ||
		bow.Properties[0].Mastery || !bow.Properties[1].Mastery {
		t.Fatalf("longbow = %+v", bow)
	}
	plate := snap.Armor[0]
	if plate.ACBase != 18 || plate.DexCap != nil || *plate.StrengthRequired != 15 || !plate.StealthDisadvantage {
		t.Fatalf("plate = %+v", plate)
	}
	assertGear(t, snap)
	assertGoblin(t, snap.Monsters[0])
}

func assertGear(t *testing.T, snap snapshot.Snapshot) {
	t.Helper()
	rope, odd, bag := snap.Items[0], snap.Items[1], snap.Items[2]
	if rope.CostGP != 1 || rope.WeightLB != 5 || rope.Magic || odd.CostGP != 0 || odd.WeightLB != 0 {
		t.Fatalf("items = %+v %+v", rope, odd)
	}
	if !bag.Magic || bag.Rarity != "uncommon" || !bag.RequiresAttunement || bag.AttunementDetail != "by a wizard" {
		t.Fatalf("bag = %+v", bag)
	}
}

func assertGoblin(t *testing.T, g snapshot.Monster) {
	t.Helper()
	if g.ChallengeRating != 0.25 || g.Abilities["dexterity"] != 14 || g.Saves["dexterity"] != 4 || len(g.Saves) != 1 ||
		g.Skills["sleight-of-hand"] != 6 || len(g.Skills) != 1 || g.Speeds["walk"] != 30 || len(g.Speeds) != 1 ||
		g.Senses["darkvision"] != 60 || len(g.Senses) != 1 || g.Resistances[0] != "fire" || g.ConditionImmunities[0] != "charmed" ||
		g.Languages != "Common, Goblin" || g.Traits[0].Name != "Nimble" {
		t.Fatalf("goblin = %+v", g)
	}
	scimitar, spit := g.Actions[0].Attacks[0], g.Actions[1].Attacks[0]
	if g.Actions[0].Type != "action" || scimitar.DamageDice != "1d6" || scimitar.DamageBonus != 2 || scimitar.DamageType != "slashing" ||
		scimitar.ExtraDice != "1d4" || scimitar.ExtraType != "poison" || scimitar.ReachFeet != 5 || scimitar.Kind != "weapon" {
		t.Fatalf("scimitar = %+v", scimitar)
	}
	if spit.DamageDice != "" || spit.DamageType != "acid" || spit.ExtraType != "" || spit.RangeFeet != 30 || spit.LongRangeFeet != 60 {
		t.Fatalf("spit = %+v", spit)
	}
}

func TestFetchSurfacesEntryErrors(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"/v2/classes/", "/v2/species/", "/v2/backgrounds/", "/v2/feats/", "/v2/weapons/", "/v2/armor/", "/v2/items/", "/v2/magicitems/", "/v2/creatures/"} {
		routes := allRoutes()
		delete(routes, missing)
		srv := server(t, routes)
		if _, err := (open5e.Client{BaseURL: srv.URL, HTTP: srv.Client()}).Fetch(context.Background()); err == nil {
			t.Errorf("missing %s ignored", missing)
		}
	}
}

func TestFetchRetriesServerErrors(t *testing.T) {
	t.Parallel()
	routes := allRoutes()
	var calls atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/conditions/" && calls.Add(1) == 1 {
			http.Error(w, "flaky", http.StatusBadGateway)
			return
		}
		key := r.URL.Path
		if r.URL.Query().Get("page") != "" {
			key += "?page=" + r.URL.Query().Get("page")
		}
		_, _ = fmt.Fprint(w, strings.ReplaceAll(routes[key], "BASE", srv.URL))
	}))
	t.Cleanup(srv.Close)
	if _, err := (open5e.Client{BaseURL: srv.URL, HTTP: srv.Client(), Backoff: time.Millisecond}).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("conditions fetched %d times", calls.Load())
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
	noConditions := server(t, map[string]string{"/v2/spells/": empty})
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
