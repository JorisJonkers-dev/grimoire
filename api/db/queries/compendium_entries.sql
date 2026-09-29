-- name: UpsertClass :one
INSERT INTO compendium.classes (document_id, slug, name, description, parent_slug, hit_die, caster_type)
VALUES (@document_id, @slug, @name, @description, sqlc.narg(parent_slug), sqlc.narg(hit_die), @caster_type)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description,
    parent_slug = EXCLUDED.parent_slug, hit_die = EXCLUDED.hit_die, caster_type = EXCLUDED.caster_type
RETURNING id;

-- name: ClearClassChildren :exec
WITH s AS (DELETE FROM compendium.class_saving_throws WHERE class_saving_throws.class_id = @class_id)
DELETE FROM compendium.class_features WHERE class_features.class_id = @class_id;

-- name: AddClassSave :exec
INSERT INTO compendium.class_saving_throws (class_id, ability_id)
SELECT @class_id, id FROM compendium.ability_scores WHERE slug = @ability ON CONFLICT DO NOTHING;

-- name: AddClassFeature :one
INSERT INTO compendium.class_features (class_id, slug, name, description, ordering)
VALUES (@class_id, @slug, @name, @description, @ordering)
ON CONFLICT (class_id, slug) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description
RETURNING id;

-- name: AddClassFeatureLevel :exec
INSERT INTO compendium.class_feature_levels (feature_id, level) VALUES (@feature_id, @level) ON CONFLICT DO NOTHING;

-- name: UpsertSpecies :one
INSERT INTO compendium.species (document_id, slug, name, description, subspecies)
VALUES (@document_id, @slug, @name, @description, @subspecies)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, subspecies = EXCLUDED.subspecies
RETURNING id;

-- name: ClearSpeciesTraits :exec
DELETE FROM compendium.species_traits WHERE species_id = @species_id;

-- name: AddSpeciesTrait :exec
INSERT INTO compendium.species_traits (species_id, ordering, name, description) VALUES (@species_id, @ordering, @name, @description);

-- name: UpsertBackground :one
INSERT INTO compendium.backgrounds (document_id, slug, name, description) VALUES (@document_id, @slug, @name, @description)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description
RETURNING id;

-- name: ClearBackgroundBenefits :exec
DELETE FROM compendium.background_benefits WHERE background_id = @background_id;

-- name: AddBackgroundBenefit :exec
INSERT INTO compendium.background_benefits (background_id, ordering, name, description) VALUES (@background_id, @ordering, @name, @description);

-- name: UpsertFeat :one
INSERT INTO compendium.feats (document_id, slug, name, description, feat_type, prerequisite)
VALUES (@document_id, @slug, @name, @description, @feat_type, @prerequisite)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description,
    feat_type = EXCLUDED.feat_type, prerequisite = EXCLUDED.prerequisite
RETURNING id;

-- name: ClearFeatBenefits :exec
DELETE FROM compendium.feat_benefits WHERE feat_id = @feat_id;

-- name: AddFeatBenefit :exec
INSERT INTO compendium.feat_benefits (feat_id, ordering, description) VALUES (@feat_id, @ordering, @description);

-- name: UpsertWeapon :one
INSERT INTO compendium.weapons (document_id, slug, name, damage_dice, damage_type_id, range_feet, long_range_feet, simple)
VALUES (@document_id, @slug, @name, @damage_dice, sqlc.narg(damage_type_id), @range_feet, @long_range_feet, @simple)
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, damage_dice = EXCLUDED.damage_dice,
    damage_type_id = EXCLUDED.damage_type_id, range_feet = EXCLUDED.range_feet, long_range_feet = EXCLUDED.long_range_feet,
    simple = EXCLUDED.simple
RETURNING id;

-- name: ClearWeaponProperties :exec
DELETE FROM compendium.weapon_properties WHERE weapon_id = @weapon_id;

-- name: AddWeaponProperty :exec
INSERT INTO compendium.weapon_properties (weapon_id, name, mastery, detail) VALUES (@weapon_id, @name, @mastery, sqlc.narg(detail))
ON CONFLICT DO NOTHING;

-- name: UpsertArmor :exec
INSERT INTO compendium.armor (document_id, slug, name, category, ac_base, add_dex, dex_cap, stealth_disadvantage, strength_required)
VALUES (@document_id, @slug, @name, @category, @ac_base, @add_dex, sqlc.narg(dex_cap), @stealth_disadvantage, sqlc.narg(strength_required))
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, ac_base = EXCLUDED.ac_base,
    add_dex = EXCLUDED.add_dex, dex_cap = EXCLUDED.dex_cap, stealth_disadvantage = EXCLUDED.stealth_disadvantage,
    strength_required = EXCLUDED.strength_required;

-- name: UpsertItem :exec
INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, rarity, requires_attunement, attunement_detail)
VALUES (@document_id, @slug, @name, @description, @category, @cost_gp::float8, @weight_lb::float8, @magic, sqlc.narg(rarity), @requires_attunement, sqlc.narg(attunement_detail))
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, category = EXCLUDED.category,
    cost_gp = EXCLUDED.cost_gp, weight_lb = EXCLUDED.weight_lb, magic = EXCLUDED.magic, rarity = EXCLUDED.rarity,
    requires_attunement = EXCLUDED.requires_attunement, attunement_detail = EXCLUDED.attunement_detail;

-- name: UpsertMonster :one
INSERT INTO compendium.monsters (document_id, slug, name, size, creature_type, alignment, armor_class, armor_detail, hit_points,
    hit_dice, challenge_rating, xp, strength, dexterity, constitution, intelligence, wisdom, charisma, passive_perception, languages)
VALUES (@document_id, @slug, @name, @size, @creature_type, @alignment, @armor_class, sqlc.narg(armor_detail), @hit_points,
    @hit_dice, @challenge_rating::float8, @xp, @strength, @dexterity, @constitution, @intelligence, @wisdom, @charisma, @passive_perception,
    sqlc.narg(languages))
ON CONFLICT (slug, document_id) DO UPDATE SET name = EXCLUDED.name, size = EXCLUDED.size, creature_type = EXCLUDED.creature_type,
    alignment = EXCLUDED.alignment, armor_class = EXCLUDED.armor_class, armor_detail = EXCLUDED.armor_detail,
    hit_points = EXCLUDED.hit_points, hit_dice = EXCLUDED.hit_dice, challenge_rating = EXCLUDED.challenge_rating, xp = EXCLUDED.xp,
    strength = EXCLUDED.strength, dexterity = EXCLUDED.dexterity, constitution = EXCLUDED.constitution,
    intelligence = EXCLUDED.intelligence, wisdom = EXCLUDED.wisdom, charisma = EXCLUDED.charisma,
    passive_perception = EXCLUDED.passive_perception, languages = EXCLUDED.languages
RETURNING id;

-- name: ClearMonsterChildren :exec
WITH a AS (DELETE FROM compendium.monster_stats WHERE monster_stats.monster_id = @monster_id),
     b AS (DELETE FROM compendium.monster_relations WHERE monster_relations.monster_id = @monster_id),
     c AS (DELETE FROM compendium.monster_traits WHERE monster_traits.monster_id = @monster_id)
DELETE FROM compendium.monster_actions WHERE monster_actions.monster_id = @monster_id;

-- name: AddMonsterStat :exec
INSERT INTO compendium.monster_stats (monster_id, kind, name, value) VALUES (@monster_id, @kind, @name, @value) ON CONFLICT DO NOTHING;

-- name: AddMonsterRelation :exec
INSERT INTO compendium.monster_relations (monster_id, relation, target_slug) VALUES (@monster_id, @relation, @target_slug) ON CONFLICT DO NOTHING;

-- name: AddMonsterTrait :exec
INSERT INTO compendium.monster_traits (monster_id, ordering, name, description) VALUES (@monster_id, @ordering, @name, @description);

-- name: AddMonsterAction :one
INSERT INTO compendium.monster_actions (monster_id, ordering, name, description, action_type)
VALUES (@monster_id, @ordering, @name, @description, @action_type) RETURNING id;

-- name: AddMonsterAttack :exec
INSERT INTO compendium.monster_attacks (action_id, ordering, name, kind, to_hit, reach_feet, range_feet, long_range_feet,
    damage_dice, damage_bonus, damage_type, extra_dice, extra_type)
VALUES (@action_id, @ordering, @name, @kind, @to_hit, @reach_feet, @range_feet, @long_range_feet,
    sqlc.narg(damage_dice), @damage_bonus, sqlc.narg(damage_type), sqlc.narg(extra_dice), sqlc.narg(extra_type));

-- name: ListEntries :many
WITH effective AS (
    SELECT DISTINCT ON (e.kind, e.slug) e.kind, e.id, e.slug, e.name, e.subtitle::text AS subtitle, d.key AS document_key
    FROM compendium.entries e
    JOIN compendium.documents d ON d.id = e.document_id
    WHERE e.kind = @kind AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY e.kind, e.slug, d.precedence DESC
)
SELECT kind, slug, name, subtitle::text AS subtitle, document_key FROM effective
WHERE (sqlc.narg(query)::text IS NULL OR name ILIKE '%' || sqlc.narg(query)::text || '%')
  AND (sqlc.narg(after_name)::text IS NULL OR (name, slug) > (sqlc.narg(after_name)::text, sqlc.narg(after_slug)::text))
ORDER BY name, slug
LIMIT @page_size;

-- name: FindEntry :one
SELECT e.id, e.name, e.subtitle::text AS subtitle, d.key AS document_key
FROM compendium.entries e
JOIN compendium.documents d ON d.id = e.document_id
WHERE e.kind = @kind AND e.slug = @slug AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
ORDER BY d.precedence DESC
LIMIT 1;

-- name: CountEntriesByKind :many
SELECT kind, count(*)::bigint AS total FROM compendium.entries GROUP BY kind
UNION ALL
SELECT 'spell', count(*)::bigint FROM compendium.spells
ORDER BY kind;

-- name: GetClassDetail :one
SELECT description, parent_slug, hit_die, caster_type FROM compendium.classes WHERE id = @id;

-- name: ClassSaves :many
SELECT a.slug FROM compendium.class_saving_throws cs JOIN compendium.ability_scores a ON a.id = cs.ability_id
WHERE cs.class_id = @class_id ORDER BY a.id;

-- name: ClassFeatures :many
SELECT f.name, f.description,
       coalesce((SELECT array_agg(l.level ORDER BY l.level) FROM compendium.class_feature_levels l WHERE l.feature_id = f.id), '{}')::integer[] AS levels
FROM compendium.class_features f WHERE f.class_id = @class_id ORDER BY f.ordering;

-- name: GetSpeciesDetail :one
SELECT description, subspecies FROM compendium.species WHERE id = @id;

-- name: SpeciesTraits :many
SELECT name, description FROM compendium.species_traits WHERE species_id = @species_id ORDER BY ordering;

-- name: GetBackgroundDetail :one
SELECT description FROM compendium.backgrounds WHERE id = @id;

-- name: BackgroundBenefits :many
SELECT name, description FROM compendium.background_benefits WHERE background_id = @background_id ORDER BY ordering;

-- name: GetFeatDetail :one
SELECT description, feat_type, prerequisite FROM compendium.feats WHERE id = @id;

-- name: FeatBenefits :many
SELECT description FROM compendium.feat_benefits WHERE feat_id = @feat_id ORDER BY ordering;

-- name: GetWeaponDetail :one
SELECT w.damage_dice, coalesce(dt.slug, '')::text AS damage_type, w.range_feet, w.long_range_feet, w.simple
FROM compendium.weapons w LEFT JOIN compendium.damage_types dt ON dt.id = w.damage_type_id WHERE w.id = @id;

-- name: WeaponProperties :many
SELECT name, mastery, detail FROM compendium.weapon_properties WHERE weapon_id = @weapon_id ORDER BY mastery, name;

-- name: GetArmorDetail :one
SELECT category, ac_base, add_dex, dex_cap, stealth_disadvantage, strength_required FROM compendium.armor WHERE id = @id;

-- name: GetItemDetail :one
SELECT description, category, cost_gp::float8 AS cost_gp, weight_lb::float8 AS weight_lb, magic, rarity, requires_attunement, attunement_detail
FROM compendium.items WHERE id = @id;

-- name: GetMonsterDetail :one
SELECT size, creature_type, alignment, armor_class, armor_detail, hit_points, hit_dice, challenge_rating::float8 AS challenge_rating,
       xp, strength, dexterity, constitution, intelligence, wisdom, charisma, passive_perception, languages
FROM compendium.monsters WHERE id = @id;

-- name: MonsterStats :many
SELECT kind, name, value FROM compendium.monster_stats WHERE monster_id = @monster_id ORDER BY kind, name;

-- name: MonsterRelations :many
SELECT relation, target_slug FROM compendium.monster_relations WHERE monster_id = @monster_id ORDER BY relation, target_slug;

-- name: MonsterTraits :many
SELECT name, description FROM compendium.monster_traits WHERE monster_id = @monster_id ORDER BY ordering;

-- name: MonsterActions :many
SELECT name, description, action_type FROM compendium.monster_actions WHERE monster_id = @monster_id ORDER BY ordering;

-- name: GetConditionDetail :one
SELECT description FROM compendium.conditions WHERE id = @id;
