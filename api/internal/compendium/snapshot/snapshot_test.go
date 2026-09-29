package snapshot_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

const valid = `{"source":"test","documents":[{"key":"srd-2024","title":"SRD 5.2","rulesetYear":2024,"precedence":20,"license":"CC-BY-4.0","attribution":"a","url":"https://x"}],
"spells":[{"document":"srd-2024","slug":"fire-bolt","name":"Fire Bolt","level":0,"school":"evocation","classes":[],"damageTypes":[],"scaling":[]}],
"conditions":[{"document":"srd-2024","slug":"prone","name":"Prone","description":"On the ground."}]}`

func TestLoadValidSnapshot(t *testing.T) {
	t.Parallel()
	s, hash, err := snapshot.Load(fstest.MapFS{snapshot.File: {Data: []byte(valid)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Spells) != 1 || len(s.Conditions) != 1 || len(hash) != 64 {
		t.Fatalf("loaded %+v %q", s, hash)
	}
	_, again, _ := snapshot.Load(fstest.MapFS{snapshot.File: {Data: []byte(valid)}})
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
		t.Fatal("bad json accepted")
	}
}

func TestValidateCatchesEachInvariant(t *testing.T) {
	t.Parallel()
	doc := snapshot.Document{Key: "d", Title: "D", License: "L", Attribution: "A"}
	spell := snapshot.Spell{Document: "d", Slug: "ok", Name: "Ok", School: "evocation"}
	cases := map[string]snapshot.Snapshot{
		"incomplete document": {Documents: []snapshot.Document{{Key: "d"}}},
		"unknown document":    {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "x", Slug: "ok", Name: "Ok", School: "s"}}},
		"bad slug":            {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "Bad Slug", Name: "Ok", School: "s"}}},
		"bad level":           {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "ok", Name: "Ok", School: "s", Level: 10}}},
		"negative level":      {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{{Document: "d", Slug: "ok", Name: "Ok", School: "s", Level: -1}}},
		"bad condition":       {Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{spell}, Conditions: []snapshot.Condition{{Document: "d", Slug: "prone"}}},
	}
	for name, s := range cases {
		if err := s.Validate(); !errors.Is(err, snapshot.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	ok := snapshot.Snapshot{Documents: []snapshot.Document{doc}, Spells: []snapshot.Spell{spell}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}
