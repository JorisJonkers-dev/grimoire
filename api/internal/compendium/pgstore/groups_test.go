package pgstore_test

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// Every area and structural kind nests through the database at any depth: modes within modes, a
// branch within a mode and a choice within a branch, each keeping its Parts in order.
func TestStructuralPartsNestThroughTheDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	d := effects.Definition{
		Slug: "deep-storm", Name: "Deep Storm", Owner: effects.OwnedBySpell,
		Components: []effects.Component{
			effects.Area{Shape: hex.CylinderArea, SizeFt: 20, RangeFt: 120},
			effects.Choice{Modes: []effects.Mode{
				{Name: "Hail", Components: []effects.Component{
					effects.SaveCondition{Ability: "dexterity", Slug: "prone"},
					effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 20, Type: ""}, Then: []effects.Component{
						effects.Choice{Modes: []effects.Mode{
							{Name: "Freeze", Components: []effects.Component{effects.CreateSurface{Kind: surface.Grease, Rounds: 2}}},
							{Name: "Shatter", Components: []effects.Component{effects.ForcedMove{Ft: 10, Toward: false}, effects.Manual{Instruction: "Ice cracks."}}},
						}},
					}},
				}},
				{Name: "Fog", Components: []effects.Component{
					effects.Reveal{Qualities: []string{"hidden"}},
					effects.Choice{Modes: []effects.Mode{{Name: "Thick", Components: []effects.Component{effects.Summon{Monster: "mist-wolf", Count: 2, Shares: false, NeedsCommand: true}}}}},
				}},
			}},
			effects.Form{Monster: "cloud", TempHP: 6},
		},
	}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[d.Slug], d) {
		t.Fatalf("deep storm:\n got %+v\nwant %+v", got[d.Slug], d)
	}
}

// migrationsUpTo is the embedded migrations up to and including one version.
func migrationsUpTo(t *testing.T, last string) fs.FS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(db.Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() <= last+"~" {
			body, err := fs.ReadFile(db.Migrations, "migrations/"+e.Name())
			if err != nil {
				t.Fatal(err)
			}
			out["migrations/"+e.Name()] = &fstest.MapFile{Data: body}
		}
	}
	return out
}

// A choice stored before groups had names, as a row per mode with the mode's Parts beneath it, reads
// back the same once every migration has run.
func TestChoicesStoredAsModeRowsReadBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url := pgtest.EmptyURL(t)
	if err := pg.MigrateFS(ctx, url, migrationsUpTo(t, "20261014090100")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, q := range []string{
		`INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug) VALUES ('old-choice', 'Old Choice', false, 'spell', 'old-choice')`,
		`INSERT INTO compendium.effect_components (effect_id, ordinal, kind, parent)
			SELECT id, o, k, p FROM compendium.effect_definitions, (VALUES (0, 'choice', NULL), (1, 'mode', 0), (2, 'teleport', 1), (3, 'branch', 1),
				(4, 'dispel', 3), (5, 'mode', 0), (6, 'immobile', 5), (7, 'temp_hp', NULL)) AS v (o, k, p) WHERE slug = 'old-choice'`,
		`INSERT INTO compendium.effect_modes (effect_id, ordinal, name) SELECT id, o, n FROM compendium.effect_definitions, (VALUES (1, 'Away'), (5, 'Stay')) AS v (o, n) WHERE slug = 'old-choice'`,
		`INSERT INTO compendium.effect_teleports (effect_id, ordinal, range_ft) SELECT id, 2, 30 FROM compendium.effect_definitions WHERE slug = 'old-choice'`,
		`INSERT INTO compendium.effect_branches (effect_id, ordinal, condition, n, creature_type) SELECT id, 3, 'first_each_turn', 0, '' FROM compendium.effect_definitions WHERE slug = 'old-choice'`,
		`INSERT INTO compendium.effect_temp_hp (effect_id, ordinal, amount) SELECT id, 7, 5 FROM compendium.effect_definitions WHERE slug = 'old-choice'`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := pg.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	got, err := pgstore.New(pool).Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []effects.Component{
		effects.Choice{Modes: []effects.Mode{
			{Name: "Away", Components: []effects.Component{effects.Teleport{RangeFt: 30}, effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn, N: 0, Type: ""}, Then: []effects.Component{effects.Dispel{}}}}},
			{Name: "Stay", Components: []effects.Component{effects.Immobile{}}},
		}},
		effects.TempHP{Amount: 5},
	}
	if !reflect.DeepEqual(got["old-choice"].Components, want) {
		t.Fatalf("old choice = %+v", got["old-choice"].Components)
	}
}
