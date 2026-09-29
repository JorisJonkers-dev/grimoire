package snapshot_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

const valid = `{"source":"test","documents":[{"key":"srd-2024","title":"SRD 5.2","rulesetYear":2024,"precedence":20,"license":"CC-BY-4.0","attribution":"a","url":"https://x"}],
"spells":[{"document":"srd-2024","slug":"fire-bolt","name":"Fire Bolt","level":0,"school":"evocation","classes":[],"damageTypes":[],"scaling":[]}],
"conditions":[{"document":"srd-2024","slug":"prone","name":"Prone","description":"On the ground."}]}`

func gz(t *testing.T, raw string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLoadValidSnapshot(t *testing.T) {
	t.Parallel()
	s, hash, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: gz(t, valid)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Spells) != 1 || len(s.Conditions) != 1 || len(hash) != 64 {
		t.Fatalf("loaded %+v %q", s, hash)
	}
	_, again, _ := snapshot.Load(fstest.MapFS{snapshot.File: {Data: gz(t, valid)}})
	if again != hash {
		t.Fatal("hash must be stable")
	}
}

func TestLoadRejectsBrokenInput(t *testing.T) {
	t.Parallel()
	if _, _, err := snapshot.Load(fstest.MapFS{}); err == nil {
		t.Fatal("missing file accepted")
	}
	if _, _, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: []byte("{")}}); err == nil {
		t.Fatal("plain json accepted")
	}
	if _, _, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: gz(t, "{")}}); err == nil {
		t.Fatal("bad json accepted")
	}
	if _, _, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: gz(t, "{}")[:12]}}); err == nil {
		t.Fatal("truncated gzip accepted")
	}
	invalid := `{"documents":[{"key":"d"}]}`
	if _, _, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: gz(t, invalid)}}); !errors.Is(err, snapshot.ErrInvalid) {
		t.Fatalf("invalid snapshot accepted: %v", err)
	}
}

func TestValidateCatchesEachInvariant(t *testing.T) {
	t.Parallel()
	doc := snapshot.Document{Key: "d", Title: "D", License: "L", Attribution: "A"}
	spell := snapshot.Spell{Document: "d", Slug: "ok", Name: "Ok", School: "evocation"}
	bad := snapshot.Entry{Document: "d", Slug: "Bad Slug", Name: "Bad"}
	good := snapshot.Entry{Document: "d", Slug: "ok", Name: "Ok"}
	cases := map[string]snapshot.Snapshot{
		"incomplete document": {Documents: []snapshot.Document{{Key: "d"}}},
		"unknown document":    {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "x", Slug: "ok", Name: "Ok", School: "s"}}},
		"bad slug":            {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "Bad Slug", Name: "Ok", School: "s"}}},
		"bad level":           {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "ok", Name: "Ok", School: "s", Level: 10}}},
		"negative level":      {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "ok", Name: "Ok", School: "s", Level: -1}}},
		"bad condition":       {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{spell}, Conditions: []snapshot.Condition{{Document: "d", Slug: "prone"}}},
		"bad class":           {Documents: []snapshot.Document{doc}, Classes: []snapshot.Class{{Entry: bad}}},
		"bad species":         {Documents: []snapshot.Document{doc}, Species: []snapshot.Species{{Entry: bad}}},
		"bad background":      {Documents: []snapshot.Document{doc}, Backgrounds: []snapshot.Background{{Entry: bad}}},
		"bad feat":            {Documents: []snapshot.Document{doc}, Feats: []snapshot.Feat{{Entry: bad}}},
		"bad weapon":          {Documents: []snapshot.Document{doc}, Weapons: []snapshot.Weapon{{Entry: bad}}},
		"bad armor":           {Documents: []snapshot.Document{doc}, Armor: []snapshot.Armor{{Entry: bad}}},
		"bad item":            {Documents: []snapshot.Document{doc}, Items: []snapshot.Item{{Entry: bad}}},
		"bad monster":         {Documents: []snapshot.Document{doc}, Monsters: []snapshot.Monster{{Entry: bad}}},
		"entry unknown doc":   {Documents: []snapshot.Document{doc}, Feats: []snapshot.Feat{{Entry: snapshot.Entry{Document: "x", Slug: "ok", Name: "Ok"}}}},
		"entry without name":  {Documents: []snapshot.Document{doc}, Feats: []snapshot.Feat{{Entry: snapshot.Entry{Document: "d", Slug: "ok"}}}},
	}
	for name, s := range cases {
		if err := s.Validate(); !errors.Is(err, snapshot.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	ok := snapshot.Snapshot{
		Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{spell}, Classes: []snapshot.Class{{Entry: good}},
		Species: []snapshot.Species{{Entry: good}}, Backgrounds: []snapshot.Background{{Entry: good}}, Feats: []snapshot.Feat{{Entry: good}},
		Weapons: []snapshot.Weapon{{Entry: good}}, Armor: []snapshot.Armor{{Entry: good}}, Items: []snapshot.Item{{Entry: good}},
		Monsters: []snapshot.Monster{{Entry: good}},
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}
