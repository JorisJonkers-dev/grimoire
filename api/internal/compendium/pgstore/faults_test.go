package pgstore

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

var errInjected = errors.New("injected")

// faulty passes calls through to a real database and fails the n-th one.
type faulty struct {
	db     queries.DBTX
	calls  int
	failAt int
}

type failedRow struct{}

func (failedRow) Scan(...any) error { return errInjected }

func (f *faulty) fail() bool {
	f.calls++
	return f.calls == f.failAt
}

func (f *faulty) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.fail() {
		return pgconn.CommandTag{}, errInjected
	}
	return f.db.Exec(ctx, sql, args...)
}

func (f *faulty) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.fail() {
		return nil, errInjected
	}
	return f.db.Query(ctx, sql, args...)
}

func (f *faulty) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.fail() {
		return failedRow{}
	}
	return f.db.QueryRow(ctx, sql, args...)
}

// everyFault runs fn failing each database call in turn until a run makes fewer calls than the fault index.
func everyFault(t *testing.T, fn func(f *faulty) error) {
	t.Helper()
	for n := 1; ; n++ {
		f := &faulty{failAt: n}
		err := fn(f)
		if f.calls < n {
			if err != nil {
				t.Fatalf("clean run failed: %v", err)
			}
			return
		}
		if !errors.Is(err, errInjected) {
			t.Fatalf("fault at call %d swallowed: %v", n, err)
		}
	}
}

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
	everyFault(t, func(f *faulty) error {
		tx, err := store.Pool().Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		f.db = tx
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
	everyFault(t, func(f *faulty) error {
		f.db = store.Pool()
		_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).GetSpell(ctx, "shove", "")
		return err
	})
	for kind, slug := range map[string]string{
		"class": "fighter", "species": "dwarf", "background": "sage", "feat": "grappler", "weapon": "longbow",
		"armor": "plate-armor", "item": "rope", "monster": "goblin", "condition": "prone",
	} {
		everyFault(t, func(f *faulty) error {
			f.db = store.Pool()
			_, err := (&Store{pool: store.Pool(), q: queries.New(f)}).GetEntry(ctx, kind, slug, "")
			if errors.Is(err, compendium.ErrNotFound) {
				t.Fatalf("%s %s not found", kind, slug)
			}
			return err
		})
	}
}
