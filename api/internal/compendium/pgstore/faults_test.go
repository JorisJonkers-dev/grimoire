package pgstore

import (
	"context"
	"errors"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func sample() snapshot.Snapshot {
	s := snapshot.Snapshot{
		Documents: []snapshot.Document{
			{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"},
			{Key: "srd-2014", Title: "SRD 5.1", RulesetYear: 2014, Precedence: 10, License: "CC-BY-4.0", Attribution: "b", URL: "https://b"},
		},
		Conditions: []snapshot.Condition{{Document: "srd-2024", Slug: "prone", Name: "Prone", Description: "Down."}},
		Spells: []snapshot.Spell{{
			Document: "srd-2024", Slug: "shove", Name: "Shove", School: "evocation", Description: "Knocked prone.",
			Classes: []string{"wizard"}, DamageTypes: []string{"force"}, SaveAbility: "strength",
			Scaling: []snapshot.Scaling{{Kind: "slot", Level: 2, DamageRoll: "1d6"}},
		}},
	}
	AddSampleEntries(&s)
	return s
}

func TestEveryImportFaultAborts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		tx, err := store.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		f.DB = tx
		return importInto(ctx, queries.New(f), sample(), "h")
	})
}

func TestEveryPresenterFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	s := New(store.Pool())
	if _, err := s.Import(ctx, sample(), "h"); err != nil {
		t.Fatal(err)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		f.DB = store.Pool()
		_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).GetSpell(ctx, "shove", "")
		return err
	})
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		f.DB = store.Pool()
		_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).BuilderOptions(ctx, "srd-2024")
		return err
	})
	for kind, slug := range map[string]string{
		"class": "fighter", "species": "dwarf", "background": "sage", "feat": "grappler", "weapon": "longbow",
		"armor": "plate-armor", "item": "rope", "monster": "goblin", "condition": "prone",
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			f.DB = store.Pool()
			_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).GetEntry(ctx, kind, slug, "")
			if errors.Is(err, compendium.ErrNotFound) {
				t.Fatalf("%s %s not found", kind, slug)
			}
			return err
		})
	}
}

func TestEveryEffectFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	s := New(store.Pool())
	seeded, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	every := func(f func(slug string, d effects.Definition)) {
		for slug, d := range seeded {
			f(slug, d)
		}
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		f.DB = store.Pool()
		_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).Effects(ctx)
		return err
	})
	every(func(slug string, d effects.Definition) {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			tx, err := store.Pool().Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			f.DB = tx
			return saveEffect(ctx, queries.New(f), effects.Owner{Kind: effects.OwnedBySpell, Slug: slug}, d)
		})
	})
}
