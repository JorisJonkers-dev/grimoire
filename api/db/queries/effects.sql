-- name: UpsertEffectDefinition :one
INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
VALUES (@slug, @name, @concentration, @owner_kind, @owner_slug)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, concentration = EXCLUDED.concentration,
    owner_kind = EXCLUDED.owner_kind, owner_slug = EXCLUDED.owner_slug
RETURNING id;

-- name: ClearEffectComponents :exec
DELETE FROM compendium.effect_components WHERE effect_id = @effect_id;

-- name: InsertEffectComponent :exec
INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (@effect_id, @ordinal, @kind);

-- name: InsertEffectBonusDie :exec
INSERT INTO compendium.effect_bonus_dice (effect_id, ordinal, dice, on_attacks, on_saves)
VALUES (@effect_id, @ordinal, @dice, @on_attacks, @on_saves);

-- name: InsertEffectEdge :exec
INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach)
VALUES (@effect_id, @ordinal, @against, @advantage, @reach);

-- name: InsertEffectExtraDamage :exec
INSERT INTO compendium.effect_extra_damage (effect_id, ordinal, dice) VALUES (@effect_id, @ordinal, @dice);

-- name: InsertEffectMoveCost :exec
INSERT INTO compendium.effect_move_costs (effect_id, ordinal, multiplier) VALUES (@effect_id, @ordinal, @multiplier);

-- name: InsertEffectManual :exec
INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (@effect_id, @ordinal, @instruction);

-- name: InsertEffectArea :exec
INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft)
VALUES (@effect_id, @ordinal, @shape, @size_ft, @range_ft);

-- name: InsertEffectSaveDamage :exec
INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half)
VALUES (@effect_id, @ordinal, @ability, @dice, @damage_type, @half);

-- name: InsertEffectSaveCondition :exec
INSERT INTO compendium.effect_save_conditions (effect_id, ordinal, ability, condition_slug)
VALUES (@effect_id, @ordinal, @ability, @condition_slug);

-- name: InsertEffectSurface :exec
INSERT INTO compendium.effect_surfaces (effect_id, ordinal, surface, rounds) VALUES (@effect_id, @ordinal, @surface, @rounds);

-- name: ListEffectDefinitions :many
SELECT id, slug, name, concentration FROM compendium.effect_definitions ORDER BY slug;

-- name: ListEffectComponents :many
SELECT effect_id, ordinal, kind FROM compendium.effect_components ORDER BY effect_id, ordinal;

-- name: ListEffectBonusDice :many
SELECT effect_id, ordinal, dice, on_attacks, on_saves FROM compendium.effect_bonus_dice;

-- name: ListEffectEdges :many
SELECT effect_id, ordinal, against, advantage, reach FROM compendium.effect_edges;

-- name: ListEffectExtraDamage :many
SELECT effect_id, ordinal, dice FROM compendium.effect_extra_damage;

-- name: ListEffectMoveCosts :many
SELECT effect_id, ordinal, multiplier FROM compendium.effect_move_costs;

-- name: ListEffectManual :many
SELECT effect_id, ordinal, instruction FROM compendium.effect_manual;

-- name: ListEffectAreas :many
SELECT effect_id, ordinal, shape, size_ft, range_ft FROM compendium.effect_areas;

-- name: ListEffectSaveDamage :many
SELECT effect_id, ordinal, ability, dice, damage_type, half FROM compendium.effect_save_damage;

-- name: ListEffectSaveConditions :many
SELECT effect_id, ordinal, ability, condition_slug FROM compendium.effect_save_conditions;

-- name: ListEffectSurfaces :many
SELECT effect_id, ordinal, surface, rounds FROM compendium.effect_surfaces;
