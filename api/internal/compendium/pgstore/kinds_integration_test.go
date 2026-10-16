//go:build integration

package pgstore

import (
	"context"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Every kind the rules define has a store of its own and a row in the lookup table of kinds; the
// lookup table, not a shared check, decides which kinds a Part row may name.
func TestEveryKindHasItsStoreAndItsRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	var defined, stored []string
	for _, k := range effects.Kinds() {
		defined = append(defined, k.Describe().Slug)
	}
	for _, r := range kindRows() {
		stored = append(stored, r.kind())
	}
	if !reflect.DeepEqual(stored, defined) {
		t.Fatalf("stores %v for kinds %v", stored, defined)
	}
	for _, slug := range defined {
		var n int
		if err := store.Pool().QueryRow(ctx, "SELECT count(*) FROM compendium.effect_kinds WHERE slug = $1", slug).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s in the lookup table = %d, %v", slug, n, err)
		}
	}
	var checks int
	if err := store.Pool().QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE conname = 'effect_components_kind_check'").Scan(&checks); err != nil || checks != 0 {
		t.Fatalf("the shared check is gone = %d, %v", checks, err)
	}

	tx, err := store.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
		VALUES ('kinds-probe', 'Kinds Probe', false, 'spell', 'kinds-probe') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT unknown"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES ($1, 0, 'heal')", id); err == nil {
		t.Fatal("a kind missing from the lookup table was stored")
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT unknown"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO compendium.effect_kinds (slug) VALUES ('heal')"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES ($1, 0, 'heal')", id); err != nil {
		t.Fatalf("a new kind needs only its own row: %v", err)
	}
}
