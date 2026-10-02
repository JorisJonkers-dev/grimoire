-- name: RulesetYear :one
SELECT ruleset_year FROM compendium.documents WHERE key = $1;

-- name: BuilderClasses :many
SELECT c.slug, c.name, coalesce(c.hit_die, 0)::int AS hit_die,
       coalesce(array_agg(a.slug ORDER BY a.id) FILTER (WHERE a.slug IS NOT NULL), '{}')::text[] AS saves
FROM compendium.classes c
JOIN compendium.documents d ON d.id = c.document_id
LEFT JOIN compendium.class_saving_throws st ON st.class_id = c.id
LEFT JOIN compendium.ability_scores a ON a.id = st.ability_id
WHERE d.key = $1 AND c.parent_slug IS NULL
GROUP BY c.id
ORDER BY c.name;

-- name: BuilderSpecies :many
SELECT s.slug, s.name,
       coalesce((SELECT t.description FROM compendium.species_traits t WHERE t.species_id = s.id AND t.name = 'Speed' LIMIT 1), '')::text AS speed
FROM compendium.species s
JOIN compendium.documents d ON d.id = s.document_id
WHERE d.key = $1 AND NOT s.subspecies
ORDER BY s.name;

-- name: BuilderBackgrounds :many
SELECT b.slug, b.name,
       coalesce((SELECT x.description FROM compendium.background_benefits x WHERE x.background_id = b.id AND x.name = 'Ability Scores' LIMIT 1), '')::text AS abilities,
       coalesce((SELECT x.description FROM compendium.background_benefits x WHERE x.background_id = b.id AND x.name = 'Skill Proficiencies' LIMIT 1), '')::text AS skills
FROM compendium.backgrounds b
JOIN compendium.documents d ON d.id = b.document_id
WHERE d.key = $1
ORDER BY b.name;

-- name: BuilderArmor :many
SELECT a.slug, a.name, a.category, a.ac_base, a.add_dex, a.dex_cap, a.stealth_disadvantage, a.strength_required
FROM compendium.armor a
JOIN compendium.documents d ON d.id = a.document_id
WHERE d.key = $1
ORDER BY a.ac_base, a.name;

-- name: BuilderWeapons :many
SELECT w.slug, w.name, w.damage_dice, coalesce(dt.slug, '')::text AS damage_type, w.simple, w.range_feet, w.long_range_feet,
       ARRAY(SELECT p.name FROM compendium.weapon_properties p WHERE p.weapon_id = w.id ORDER BY p.name)::text[] AS properties
FROM compendium.weapons w
JOIN compendium.documents d ON d.id = w.document_id
LEFT JOIN compendium.damage_types dt ON dt.id = w.damage_type_id
WHERE d.key = $1
ORDER BY w.simple DESC, w.name;

-- name: SheetClassFeatures :many
-- A class's features up to a level, each at the first level it is gained.
SELECT f.name, f.description, min(l.level)::integer AS level
FROM compendium.class_features f
JOIN compendium.classes c ON c.id = f.class_id
JOIN compendium.documents d ON d.id = c.document_id
JOIN compendium.class_feature_levels l ON l.feature_id = f.id
WHERE d.key = @ruleset AND c.slug = @class AND l.level <= @level
GROUP BY f.id, f.name, f.description, f.ordering
ORDER BY min(l.level), f.ordering;

-- name: SheetSpeciesTraits :many
SELECT t.name, t.description FROM compendium.species_traits t
JOIN compendium.species s ON s.id = t.species_id
JOIN compendium.documents d ON d.id = s.document_id
WHERE d.key = @ruleset AND s.slug = @species
ORDER BY t.ordering;

-- name: LevelUpSubclasses :many
SELECT c.slug, c.name FROM compendium.classes c
JOIN compendium.documents d ON d.id = c.document_id
WHERE d.key = @ruleset AND c.parent_slug = @class
ORDER BY c.name;

-- name: LevelUpFeats :many
SELECT f.slug, f.name, f.feat_type, f.description FROM compendium.feats f
JOIN compendium.documents d ON d.id = f.document_id
WHERE d.key = @ruleset
ORDER BY f.name;

-- name: LevelUpSpells :many
-- A class's cantrips and spells up to a spell level.
SELECT s.slug, s.name, s.level, s.ritual, s.casting_time FROM compendium.spells s
JOIN compendium.documents d ON d.id = s.document_id
JOIN compendium.spell_classes sc ON sc.spell_id = s.id
WHERE d.key = @ruleset AND sc.class_slug = @class AND s.level <= @max_level
ORDER BY s.level, s.name;

-- name: AlwaysPreparedSpells :many
-- The spells a class and its subclass always have prepared at a class level.
SELECT s.slug, s.name, s.level, s.ritual, s.casting_time FROM compendium.always_prepared a
JOIN compendium.spells s ON s.slug = a.spell_slug
JOIN compendium.documents d ON d.id = s.document_id
WHERE d.key = @ruleset AND a.level <= @level
    AND ((a.owner_kind = 'class' AND a.owner_slug = @class) OR (a.owner_kind = 'subclass' AND a.owner_slug = @subclass))
ORDER BY s.level, s.name;
