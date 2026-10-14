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
		Parts: []effects.Part{
			effects.Area{Shape: hex.CylinderArea, SizeFt: 20, RangeFt: 120},
			effects.Choice{Modes: []effects.Group{
				{Name: "Hail", Parts: []effects.Part{
					effects.SaveCondition{Ability: "dexterity", Slug: "prone"},
					effects.Branch{When: effects.Condition{Kind: effects.HPAtMost, N: 20, Type: ""}, Then: []effects.Part{
						effects.Choice{Modes: []effects.Group{
							{Name: "Freeze", Parts: []effects.Part{effects.CreateSurface{Surface: surface.Grease, Rounds: 2}}},
							{Name: "Shatter", Parts: []effects.Part{effects.ForcedMove{Ft: 10, Toward: false}, effects.Manual{Instruction: "Ice cracks."}}},
						}},
					}},
				}},
				{Name: "Fog", Parts: []effects.Part{
					effects.Reveal{Qualities: []string{"hidden"}},
					effects.Choice{Modes: []effects.Group{{Name: "Thick", Parts: []effects.Part{effects.Summon{Monster: "mist-wolf", Count: 2, Shares: false, NeedsCommand: true}}}}},
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
	want := []effects.Part{
		effects.Choice{Modes: []effects.Group{
			{Name: "Away", Parts: []effects.Part{effects.Teleport{RangeFt: 30}, effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn, N: 0, Type: ""}, Then: []effects.Part{effects.Dispel{}}}}},
			{Name: "Stay", Parts: []effects.Part{effects.Immobile{}}},
		}},
		effects.TempHP{Amount: 5},
	}
	if !reflect.DeepEqual(got["old-choice"].Parts, want) {
		t.Fatalf("old choice = %+v", got["old-choice"].Parts)
	}
}

// A stacking Part keeps the highest level it rises to through the database.
func TestStackingPartsKeepTheirCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := pgstore.New(openPool(t))
	d := effects.Definition{Slug: "frostbite", Name: "Frostbite", Owner: effects.OwnedByCondition, Parts: []effects.Part{
		effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 0, MaxLevel: 3},
	}}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedByCondition, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[d.Slug], d) || got.MaxLevel(d.Slug) != 3 {
		t.Fatalf("frostbite = %+v, rises to %d", got[d.Slug], got.MaxLevel(d.Slug))
	}
	if ex := got["exhaustion"].Parts; !reflect.DeepEqual(ex, []effects.Part{effects.Exhausting{D20PerLevel: 2, SpeedFtPerLevel: 5, DeathAt: 6, MaxLevel: 0}}) {
		t.Fatalf("stored exhaustion keeps no cap = %+v", ex)
	}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedByCondition, Slug: "too-high"}, effects.Definition{Slug: "too-high", Name: "Too High", Parts: []effects.Part{
		effects.Exhausting{D20PerLevel: 1, MaxLevel: 11},
	}}); err == nil {
		t.Fatal("a cap above 10 was stored")
	}
}

// A Part beneath another names its group in its own row: a mode is a named group of the choice, not a
// row of its own, and a group of no Parts is refused rather than lost.
func TestGroupsOfPartsHaveNames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := openPool(t)
	s := pgstore.New(pool)
	d := effects.Definition{Slug: "lantern-choice", Name: "Lantern Choice", Owner: effects.OwnedBySpell, Parts: []effects.Part{
		effects.Area{Shape: hex.EmanationArea, SizeFt: 10, RangeFt: 0},
		effects.AreaSave{Ability: "wisdom"},
		effects.Choice{Modes: []effects.Group{
			{Name: "Bright", Parts: []effects.Part{effects.Light{BrightFt: 20, DimFt: 20}}},
			{Name: "Dim", Parts: []effects.Part{effects.Light{BrightFt: 5, DimFt: 40}, effects.Branch{
				When: effects.Condition{Kind: effects.FailsBy, N: 5, Type: ""}, Then: []effects.Part{effects.SaveCondition{Ability: "wisdom", Slug: "blinded"}},
			}}},
		}},
	}}
	if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: d.Slug}, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[d.Slug], d) {
		t.Fatalf("lantern choice:\n got %+v\nwant %+v", got[d.Slug], d)
	}
	rows, err := pool.Query(ctx, `SELECT c.kind, coalesce(c.group_name, ''), coalesce(p.kind, '') FROM compendium.effect_components c
		JOIN compendium.effect_definitions d ON d.id = c.effect_id
		LEFT JOIN compendium.effect_components p ON p.effect_id = c.effect_id AND p.ordinal = c.parent
		WHERE d.slug = 'lantern-choice' ORDER BY c.ordinal`)
	if err != nil {
		t.Fatal(err)
	}
	var stored []string
	for rows.Next() {
		var kind, group, parent string
		if err := rows.Scan(&kind, &group, &parent); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, kind+"/"+group+"/"+parent)
	}
	want := []string{"area//", "area_save//", "choice//", "light/Bright/choice", "light/Dim/choice", "branch/Dim/choice", "save_condition/then/branch"}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("rows = %v", stored)
	}
	var modes int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM compendium.effect_kinds WHERE slug = 'mode'").Scan(&modes); err != nil || modes != 0 {
		t.Fatalf("a mode is no kind of Part = %d, %v", modes, err)
	}
	for _, bad := range []effects.Definition{
		{Slug: "empty-mode", Name: "x", Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "Hot", Parts: []effects.Part{effects.Dispel{}}}, {Name: "Cold", Parts: nil}}}}},
		{Slug: "empty-branch", Name: "x", Parts: []effects.Part{effects.Branch{When: effects.Condition{Kind: effects.FirstEachTurn, N: 0, Type: ""}, Then: nil}}},
		{Slug: "unnamed-mode", Name: "x", Parts: []effects.Part{effects.Choice{Modes: []effects.Group{{Name: "", Parts: []effects.Part{effects.Dispel{}}}}}}},
	} {
		if err := s.SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: bad.Slug}, bad); err == nil {
			t.Errorf("%s saved", bad.Slug)
		}
	}
	for _, q := range []string{
		"UPDATE compendium.effect_components SET group_name = NULL WHERE kind = 'light' AND effect_id = (SELECT id FROM compendium.effect_definitions WHERE slug = 'lantern-choice')",
		"UPDATE compendium.effect_components SET group_name = 'x' WHERE kind = 'area' AND effect_id = (SELECT id FROM compendium.effect_definitions WHERE slug = 'lantern-choice')",
	} {
		if _, err := pool.Exec(ctx, q); err == nil {
			t.Errorf("a Part beneath another names its group, and only such a Part: %s", q)
		}
	}
}
