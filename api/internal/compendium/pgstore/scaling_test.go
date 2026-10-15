package pgstore_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// The seeded SRD spells that scale read as they always have, Scaling line last.
func TestSeededScalingReadsAsBefore(t *testing.T) {
	t.Parallel()
	got, err := pgstore.New(openPool(t)).Effects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"burning-hands":    {"A 15-foot Cone extends from the caster.", "Each creature in the area makes a Dexterity saving throw, taking 3d6 fire damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 1."},
		"thunderwave":      {"A 15-foot Cube extends from the caster.", "Each creature in the area makes a Constitution saving throw, taking 2d8 thunder damage on a failed save or half as much damage on a successful one.", "A creature that fails is pushed up to 10 feet straight away from the caster.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 1."},
		"shatter":          {"A 10-foot Sphere appears at a point within 60 feet.", "Each creature in the area makes a Constitution saving throw, taking 3d8 thunder damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 2."},
		"fireball":         {"A 20-foot Sphere appears at a point within 150 feet.", "Each creature in the area makes a Dexterity saving throw, taking 8d6 fire damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3."},
		"lightning-bolt":   {"A 100-foot Line extends from the caster.", "Each creature in the area makes a Dexterity saving throw, taking 8d6 lightning damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d6 for each slot level above 3."},
		"cone-of-cold":     {"A 60-foot Cone extends from the caster.", "Each creature in the area makes a Constitution saving throw, taking 8d8 cold damage on a failed save or half as much damage on a successful one.", "Duration: Instantaneous.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 5."},
		"spirit-guardians": {"A 15-foot Emanation surrounds the caster and moves with it.", "Each creature in the area makes a Wisdom saving throw, taking 3d8 radiant damage on a failed save or half as much damage on a successful one.", "Duration: Concentration, up to 10 minutes.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 3."},
		"wall-of-fire":     {"A 60-foot Wall appears at a point within 120 feet.", "Each creature in the area makes a Dexterity saving throw, taking 5d8 fire damage on a failed save or half as much damage on a successful one.", "Duration: Concentration, up to 1 minute.", "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 4."},
	}
	for slug, lines := range want {
		if text := got[slug].Text(); !reflect.DeepEqual(text, lines) {
			t.Errorf("%s reads\n%q\nwant\n%q", slug, text, lines)
		}
	}
}

// legacyScaling is every row of the one-Scaling-per-Effect tables, in order, as text.
func legacyScaling(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT format('%s %s %s %s %s %s', d.slug, s.axis, s.class_slug, s.column_name, s.base_level, s.dice) FROM compendium.effect_scalings s
		JOIN compendium.effect_definitions d ON d.id = s.effect_id
		UNION ALL SELECT format('%s step %s %s', d.slug, st.at_level, st.dice) FROM compendium.effect_scaling_steps st
		JOIN compendium.effect_definitions d ON d.id = st.effect_id ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Scaling stored as the one record an Effect had reads back as the same rule over the dice of its area
// damage once every migration has run, with the same rules text; the old rows stay as they were, so an
// older binary still reads them.
func TestScalingStoredAsOneRowPerEffectReadsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url := pgtest.EmptyURL(t)
	if err := pg.MigrateFS(ctx, url, migrationsUpTo(t, "20261015090100")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, q := range []string{
		`INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
			SELECT s, s, false, 'spell', s FROM unnest(ARRAY['old-slot', 'old-cantrip', 'old-aura', 'old-sneak', 'old-heal', 'old-blank']) AS s`,
		`INSERT INTO compendium.effect_components (effect_id, ordinal, kind) SELECT id, o, k FROM compendium.effect_definitions,
			(VALUES (0, 'area'), (1, 'save_damage'), (2, 'save_damage')) AS v (o, k) WHERE slug LIKE 'old-%' AND slug <> 'old-heal'`,
		`INSERT INTO compendium.effect_components (effect_id, ordinal, kind) SELECT id, 0, 'temp_hp' FROM compendium.effect_definitions WHERE slug = 'old-heal'`,
		`INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) SELECT id, 0, 'sphere', 20, 60 FROM compendium.effect_definitions WHERE slug LIKE 'old-%' AND slug <> 'old-heal'`,
		`INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) SELECT id, o, 'dexterity', '1d10', 'fire', true
			FROM compendium.effect_definitions, (VALUES (1), (2)) AS v (o) WHERE slug LIKE 'old-%' AND slug <> 'old-heal'`,
		`INSERT INTO compendium.effect_temp_hp (effect_id, ordinal, amount) SELECT id, 0, 9 FROM compendium.effect_definitions WHERE slug = 'old-heal'`,
		`INSERT INTO compendium.effect_scalings (effect_id, axis, class_slug, column_name, base_level, dice) SELECT id, a, c, n, b, x FROM compendium.effect_definitions,
			(VALUES ('old-slot', 'slot_level', '', '', 2, '1d8'), ('old-cantrip', 'character_level', '', '', 0, ''), ('old-aura', 'class_level', 'paladin', '', 0, ''),
				('old-sneak', 'table_column', 'rogue', 'Sneak Attack', 0, ''), ('old-heal', 'slot_level', '', '', 1, '1d4'), ('old-blank', 'slot_level', '', '', 1, ''))
			AS v (s, a, c, n, b, x) WHERE slug = s`,
		`INSERT INTO compendium.effect_scaling_steps (effect_id, at_level, dice) SELECT id, l, x FROM compendium.effect_definitions,
			(VALUES ('old-cantrip', 5, '2d10'), ('old-cantrip', 11, '3d10'), ('old-aura', 18, '2d8'), ('old-slot', 9, '4d8')) AS v (s, l, x) WHERE slug = s`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	before := legacyScaling(ctx, t, pool)
	if err := pg.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	if after := legacyScaling(ctx, t, pool); !reflect.DeepEqual(after, before) || len(after) != 18 {
		t.Fatalf("old rows changed:\n%q\n%q", after, before)
	}
	got, err := pgstore.New(pool).Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dice := func(a effects.Axis, class string, r effects.Rule) []effects.Scaling {
		return []effects.Scaling{{Part: 2, Field: "dice", Axis: a, Class: class, Rule: r}}
	}
	steps := func(st ...effects.Step) effects.Rule { return effects.Table{Steps: st} }
	want := map[string][]effects.Scaling{
		"old-slot":    dice(effects.SlotLevel, "", effects.Linear{Base: 2, Per: effects.Value{Text: "1d8"}}),
		"old-cantrip": dice(effects.CharacterLevel, "", steps(effects.Step{At: 5, Value: effects.Value{Text: "2d10"}}, effects.Step{At: 11, Value: effects.Value{Text: "3d10"}})),
		"old-aura":    dice(effects.ClassLevel, "paladin", steps(effects.Step{At: 18, Value: effects.Value{Text: "2d8"}})),
		"old-sneak":   dice(effects.TableColumn, "rogue", effects.Column{Name: "Sneak Attack"}),
		"old-heal":    nil,
		"old-blank":   nil,
	}
	text := map[string]string{
		"old-slot":    "Using a Higher-Level Spell Slot: the damage increases by 1d8 for each slot level above 2.",
		"old-cantrip": "Cantrip Upgrade: the damage becomes 2d10 at level 5, 3d10 at level 11.",
		"old-aura":    "The damage becomes 2d8 at Paladin level 18.",
		"old-sneak":   "The damage equals the Sneak Attack column of the Rogue table.",
	}
	for slug, sc := range want {
		d := got[slug]
		if !reflect.DeepEqual(d.Scaling, sc) {
			t.Errorf("%s scales %+v", slug, d.Scaling)
		}
		if lines := d.Text(); text[slug] != "" && lines[len(lines)-1] != text[slug] {
			t.Errorf("%s reads %q", slug, lines)
		}
	}
	if area, _ := got.AreaAt("old-slot", effects.Level{Slot: 4}); area.Damage.Dice != "1d10+1d8+1d8" {
		t.Fatalf("old slot at 4 = %+v", area.Damage)
	}
}

// The database refuses Scaling rows that cannot be right: a rule over a Part the Effect does not have, a
// rule's own values missing or doubled, and a step that holds no value or two.
func TestScalingRowsAreChecked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := openPool(t)
	var id int64
	if err := pool.QueryRow(ctx, `SELECT id FROM compendium.effect_definitions WHERE slug = 'fireball'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO compendium.effect_scaling_rules (effect_id, position, part, field, axis, class_slug, rule, base_level, per_dice, per_number, formula, column_name)
		VALUES ($1, 5, %s, 'dice', '%s', '', '%s', 3, %s, %s, %s, '%s')`
	for name, q := range map[string]string{
		"a Part it does not have":  fmt.Sprintf(insert, "7", "slot_level", "linear", "'1d6'", "NULL", "NULL", ""),
		"a linear rule of nothing": fmt.Sprintf(insert, "1", "slot_level", "linear", "NULL", "NULL", "NULL", ""),
		"a linear rule of two":     fmt.Sprintf(insert, "1", "slot_level", "linear", "'1d6'", "1", "NULL", ""),
		"a table with a per":       fmt.Sprintf(insert, "1", "slot_level", "table", "NULL", "1", "NULL", ""),
		"a formula of nothing":     fmt.Sprintf(insert, "1", "slot_level", "formula", "NULL", "NULL", "NULL", ""),
		"a table with a formula":   fmt.Sprintf(insert, "1", "slot_level", "table", "NULL", "NULL", "'level'", ""),
		"a column by slot":         fmt.Sprintf(insert, "1", "slot_level", "column", "NULL", "NULL", "NULL", "Sneak Attack"),
		"a table column by table":  fmt.Sprintf(insert, "1", "table_column", "table", "NULL", "NULL", "NULL", ""),
		"a formula with code":      fmt.Sprintf(insert, "1", "slot_level", "formula", "NULL", "NULL", "'pg_sleep(1)'", ""),
		"a step of no value":       `INSERT INTO compendium.effect_scaling_rule_steps (effect_id, position, at_level) SELECT effect_id, 0, 5 FROM compendium.effect_scaling_rules WHERE effect_id = $1`,
		"a step of two values":     `INSERT INTO compendium.effect_scaling_rule_steps (effect_id, position, at_level, value_dice, value_number) SELECT effect_id, 0, 5, '1d6', 1 FROM compendium.effect_scaling_rules WHERE effect_id = $1`,
	} {
		if _, err := pool.Exec(ctx, q, id); err == nil {
			t.Errorf("%s saved", name)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(insert, "1", "slot_level", "table", "NULL", "NULL", "NULL", ""), id); err != nil {
		t.Fatalf("a table over the damage: %v", err)
	}
}
