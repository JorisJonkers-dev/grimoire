-- name: UpsertEffectDefinition :one
INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount, repeat_save)
VALUES (@slug, @name, @concentration, @owner_kind, @owner_slug, sqlc.narg(duration_kind), @duration_amount, sqlc.narg(repeat_save))
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, concentration = EXCLUDED.concentration,
    owner_kind = EXCLUDED.owner_kind, owner_slug = EXCLUDED.owner_slug,
    duration_kind = EXCLUDED.duration_kind, duration_amount = EXCLUDED.duration_amount, repeat_save = EXCLUDED.repeat_save
RETURNING id;

-- name: ClearEffectComponents :exec
DELETE FROM compendium.effect_components WHERE effect_id = @effect_id;

-- name: InsertEffectComponent :exec
INSERT INTO compendium.effect_components (effect_id, ordinal, kind, parent) VALUES (@effect_id, @ordinal, @kind, sqlc.narg(parent));

-- name: InsertEffectManual :exec
INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (@effect_id, @ordinal, @instruction);

-- name: InsertEffectArea :exec
INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft)
VALUES (@effect_id, @ordinal, @shape, @size_ft, @range_ft);

-- name: InsertEffectSaveCondition :exec
INSERT INTO compendium.effect_save_conditions (effect_id, ordinal, ability, condition_slug)
VALUES (@effect_id, @ordinal, @ability, @condition_slug);

-- name: InsertEffectSurface :exec
INSERT INTO compendium.effect_surfaces (effect_id, ordinal, surface, rounds) VALUES (@effect_id, @ordinal, @surface, @rounds);

-- name: ListEffectDefinitions :many
SELECT id, slug, name, concentration, owner_kind, duration_kind, duration_amount, repeat_save FROM compendium.effect_definitions ORDER BY slug;

-- name: ListEffectComponents :many
SELECT effect_id, ordinal, kind, parent FROM compendium.effect_components ORDER BY effect_id, ordinal;

-- name: ListEffectComponentsOfKind :many
SELECT effect_id, ordinal FROM compendium.effect_components WHERE kind = @kind;

-- name: ListEffectManual :many
SELECT effect_id, ordinal, instruction FROM compendium.effect_manual;

-- name: ListEffectAreas :many
SELECT effect_id, ordinal, shape, size_ft, range_ft FROM compendium.effect_areas;

-- name: ListEffectSaveConditions :many
SELECT effect_id, ordinal, ability, condition_slug FROM compendium.effect_save_conditions;

-- name: ListEffectSurfaces :many
SELECT effect_id, ordinal, surface, rounds FROM compendium.effect_surfaces;

-- name: InsertEffectForcedMove :exec
INSERT INTO compendium.effect_forced_moves (effect_id, ordinal, ft, toward) VALUES (@effect_id, @ordinal, @ft, @toward);

-- name: ListEffectForcedMoves :many
SELECT effect_id, ordinal, ft, toward FROM compendium.effect_forced_moves;

-- name: InsertEffectMode :exec
INSERT INTO compendium.effect_modes (effect_id, ordinal, name) VALUES (@effect_id, @ordinal, @name);

-- name: ListEffectModes :many
SELECT effect_id, ordinal, name FROM compendium.effect_modes;

-- name: ClearEffectScaling :exec
DELETE FROM compendium.effect_scalings WHERE effect_id = @effect_id;

-- name: InsertEffectScaling :exec
INSERT INTO compendium.effect_scalings (effect_id, axis, class_slug, column_name, base_level, dice)
VALUES (@effect_id, @axis, @class_slug, @column_name, @base_level, @dice);

-- name: InsertEffectScalingStep :exec
INSERT INTO compendium.effect_scaling_steps (effect_id, at_level, dice) VALUES (@effect_id, @at_level, @dice);

-- name: ListEffectScalings :many
SELECT effect_id, axis, class_slug, column_name, base_level, dice FROM compendium.effect_scalings;

-- name: ListEffectScalingSteps :many
SELECT effect_id, at_level, dice FROM compendium.effect_scaling_steps ORDER BY effect_id, at_level;

-- name: InsertEffectSummon :exec
INSERT INTO compendium.effect_summons (effect_id, ordinal, monster_slug, count, shares_turn, needs_command)
VALUES (@effect_id, @ordinal, @monster_slug, @count, @shares_turn, @needs_command);

-- name: ListEffectSummons :many
SELECT effect_id, ordinal, monster_slug, count, shares_turn, needs_command FROM compendium.effect_summons;

-- name: InsertEffectForm :exec
INSERT INTO compendium.effect_forms (effect_id, ordinal, monster_slug, temp_hp) VALUES (@effect_id, @ordinal, @monster_slug, @temp_hp);

-- name: ListEffectForms :many
SELECT effect_id, ordinal, monster_slug, temp_hp FROM compendium.effect_forms;

-- name: InsertEffectReveal :exec
INSERT INTO compendium.effect_reveals (effect_id, ordinal, qualities) VALUES (@effect_id, @ordinal, @qualities::text[]);

-- name: ListEffectReveals :many
SELECT effect_id, ordinal, qualities FROM compendium.effect_reveals;

-- name: ListSurfaceDefinitions :many
SELECT slug, name, cost, obscures, hazard_dice, hazard_type, every_step, effect_slug FROM compendium.surface_definitions ORDER BY slug;

-- name: ListSurfaceReactions :many
SELECT surface, damage_type, becomes FROM compendium.surface_reactions ORDER BY surface, damage_type;

-- name: UpsertSurfaceDefinition :exec
INSERT INTO compendium.surface_definitions (slug, name, cost, obscures, hazard_dice, hazard_type, every_step, effect_slug)
VALUES (@slug, @name, @cost, sqlc.narg(obscures), sqlc.narg(hazard_dice), sqlc.narg(hazard_type), @every_step, sqlc.narg(effect_slug))
ON CONFLICT (slug) DO UPDATE SET name = excluded.name, cost = excluded.cost, obscures = excluded.obscures, hazard_dice = excluded.hazard_dice,
    hazard_type = excluded.hazard_type, every_step = excluded.every_step, effect_slug = excluded.effect_slug;

-- name: ClearSurfaceReactions :exec
DELETE FROM compendium.surface_reactions WHERE surface = @surface;

-- name: InsertSurfaceReaction :exec
INSERT INTO compendium.surface_reactions (surface, damage_type, becomes) VALUES (@surface, @damage_type, @becomes);
