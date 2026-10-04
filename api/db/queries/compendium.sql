-- name: UpsertDocument :one
INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url)
VALUES (@key, @title, @ruleset_year, @precedence, @license, @attribution, @url)
ON CONFLICT (key) DO UPDATE SET
    title = EXCLUDED.title, ruleset_year = EXCLUDED.ruleset_year, precedence = EXCLUDED.precedence,
    license = EXCLUDED.license, attribution = EXCLUDED.attribution, url = EXCLUDED.url
RETURNING id;

-- name: UpsertMagicSchool :one
INSERT INTO compendium.magic_schools (slug, name) VALUES (@slug, @name)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: UpsertDamageType :one
INSERT INTO compendium.damage_types (slug, name) VALUES (@slug, @name)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: AbilityIDBySlug :one
SELECT id FROM compendium.ability_scores WHERE slug = @slug;

-- name: UpsertSpell :one
INSERT INTO compendium.spells (
    document_id, slug, name, level, school_id, casting_time, range_text, range_feet,
    requires_verbal, requires_somatic, requires_material, material_text, ritual, concentration,
    duration, description, higher_level, save_ability_id, attack_roll, damage_roll
) VALUES (
    @document_id, @slug, @name, @level, @school_id, @casting_time, @range_text, sqlc.narg(range_feet),
    @requires_verbal, @requires_somatic, @requires_material, sqlc.narg(material_text), @ritual, @concentration,
    @duration, @description, sqlc.narg(higher_level), sqlc.narg(save_ability_id), @attack_roll, sqlc.narg(damage_roll)
)
ON CONFLICT (slug, document_id) DO UPDATE SET
    name = EXCLUDED.name, level = EXCLUDED.level, school_id = EXCLUDED.school_id,
    casting_time = EXCLUDED.casting_time, range_text = EXCLUDED.range_text, range_feet = EXCLUDED.range_feet,
    requires_verbal = EXCLUDED.requires_verbal, requires_somatic = EXCLUDED.requires_somatic,
    requires_material = EXCLUDED.requires_material, material_text = EXCLUDED.material_text,
    ritual = EXCLUDED.ritual, concentration = EXCLUDED.concentration, duration = EXCLUDED.duration,
    description = EXCLUDED.description, higher_level = EXCLUDED.higher_level,
    save_ability_id = EXCLUDED.save_ability_id, attack_roll = EXCLUDED.attack_roll, damage_roll = EXCLUDED.damage_roll
RETURNING id;

-- name: ClearSpellChildren :exec
WITH c AS (DELETE FROM compendium.spell_classes WHERE spell_classes.spell_id = @spell_id),
     d AS (DELETE FROM compendium.spell_damage_types WHERE spell_damage_types.spell_id = @spell_id)
DELETE FROM compendium.spell_scaling WHERE spell_scaling.spell_id = @spell_id;

-- name: AddSpellClass :exec
INSERT INTO compendium.spell_classes (spell_id, class_slug) VALUES (@spell_id, @class_slug)
ON CONFLICT DO NOTHING;

-- name: AddSpellDamageType :exec
INSERT INTO compendium.spell_damage_types (spell_id, damage_type_id) VALUES (@spell_id, @damage_type_id)
ON CONFLICT DO NOTHING;

-- name: AddSpellScaling :exec
INSERT INTO compendium.spell_scaling (spell_id, kind, at_level, damage_roll) VALUES (@spell_id, @kind, @at_level, @damage_roll)
ON CONFLICT DO NOTHING;

-- name: UpsertCondition :exec
INSERT INTO compendium.conditions (document_id, slug, name, description) VALUES (@document_id, @slug, @name, @description)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description;

-- name: RecordCompendiumImport :one
INSERT INTO ops.compendium_imports (snapshot_hash) VALUES (@snapshot_hash) RETURNING id;

-- name: CompendiumVersion :one
SELECT coalesce(max(id), 0)::bigint AS version FROM ops.compendium_imports;

-- name: LatestSnapshotHash :one
SELECT coalesce((SELECT snapshot_hash FROM ops.compendium_imports ORDER BY id DESC LIMIT 1), '')::text AS snapshot_hash;

-- name: ListSources :many
SELECT key, title, ruleset_year, license, attribution, url FROM compendium.documents WHERE key LIKE 'srd-%' ORDER BY precedence DESC, key;

-- name: ListSpells :many
WITH effective AS (
    SELECT DISTINCT ON (s.slug)
        s.id, s.slug, s.name, s.level, s.school_id, s.ritual, s.concentration, d.key AS document_key
    FROM compendium.spells s
    JOIN compendium.documents d ON d.id = s.document_id
    WHERE d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY s.slug, d.precedence DESC
)
SELECT e.slug, e.name, e.level, sch.slug AS school, e.ritual, e.concentration, e.document_key
FROM effective e
JOIN compendium.magic_schools sch ON sch.id = e.school_id
WHERE (sqlc.narg(query)::text IS NULL OR e.name ILIKE '%' || sqlc.narg(query)::text || '%')
  AND (sqlc.narg(level)::integer IS NULL OR e.level = sqlc.narg(level)::integer)
  AND (sqlc.narg(school)::text IS NULL OR sch.slug = sqlc.narg(school)::text)
  AND (sqlc.narg(class_slug)::text IS NULL OR EXISTS (
        SELECT 1 FROM compendium.spell_classes sc WHERE sc.spell_id = e.id AND sc.class_slug = sqlc.narg(class_slug)::text))
  AND (sqlc.narg(after_name)::text IS NULL OR (e.name, e.slug) > (sqlc.narg(after_name)::text, sqlc.narg(after_slug)::text))
ORDER BY e.name, e.slug
LIMIT @page_size;

-- name: GetSpell :one
SELECT s.id, s.slug, s.name, s.level, sch.slug AS school, d.key AS document_key, s.casting_time, s.range_text,
       s.range_feet, s.requires_verbal, s.requires_somatic, s.requires_material, s.material_text, s.ritual,
       s.concentration, s.duration, s.description, s.higher_level, ab.slug AS save_ability, s.attack_roll, s.damage_roll
FROM compendium.spells s
JOIN compendium.documents d ON d.id = s.document_id
JOIN compendium.magic_schools sch ON sch.id = s.school_id
LEFT JOIN compendium.ability_scores ab ON ab.id = s.save_ability_id
WHERE s.slug = @slug AND d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
ORDER BY d.precedence DESC
LIMIT 1;

-- name: SpellClasses :many
SELECT class_slug FROM compendium.spell_classes WHERE spell_id = @spell_id ORDER BY class_slug;

-- name: SpellDamageTypes :many
SELECT dt.slug FROM compendium.spell_damage_types sdt
JOIN compendium.damage_types dt ON dt.id = sdt.damage_type_id
WHERE sdt.spell_id = @spell_id ORDER BY dt.slug;

-- name: SpellScaling :many
SELECT kind, at_level, damage_roll FROM compendium.spell_scaling WHERE spell_id = @spell_id ORDER BY kind, at_level;

-- name: ConditionsForDocument :many
SELECT c.slug, c.name, c.description FROM compendium.conditions c
JOIN compendium.documents d ON d.id = c.document_id
WHERE d.key = @document_key ORDER BY c.name;
