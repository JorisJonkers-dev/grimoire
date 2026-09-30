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
SELECT w.slug, w.name, w.damage_dice, coalesce(dt.slug, '')::text AS damage_type, w.simple, w.range_feet, w.long_range_feet
FROM compendium.weapons w
JOIN compendium.documents d ON d.id = w.document_id
LEFT JOIN compendium.damage_types dt ON dt.id = w.damage_type_id
WHERE d.key = $1
ORDER BY w.simple DESC, w.name;
