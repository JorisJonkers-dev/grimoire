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

-- name: InsertEffectBonusDie :exec
INSERT INTO compendium.effect_bonus_dice (effect_id, ordinal, dice, on_attacks, on_saves)
VALUES (@effect_id, @ordinal, @dice, @on_attacks, @on_saves);

-- name: InsertEffectEdge :exec
INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach, source_only)
VALUES (@effect_id, @ordinal, @against, @advantage, @reach, @source_only);

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
SELECT id, slug, name, concentration, owner_kind, duration_kind, duration_amount, repeat_save FROM compendium.effect_definitions ORDER BY slug;

-- name: ListEffectComponents :many
SELECT effect_id, ordinal, kind, parent FROM compendium.effect_components ORDER BY effect_id, ordinal;

-- name: ListEffectBonusDice :many
SELECT effect_id, ordinal, dice, on_attacks, on_saves FROM compendium.effect_bonus_dice;

-- name: ListEffectEdges :many
SELECT effect_id, ordinal, against, advantage, reach, source_only FROM compendium.effect_edges;

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

-- name: InsertEffectSaveEdge :exec
INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (@effect_id, @ordinal, @ability, @mode);

-- name: InsertEffectCrit :exec
INSERT INTO compendium.effect_crits (effect_id, ordinal, feet) VALUES (@effect_id, @ordinal, @feet);

-- name: InsertEffectExhaustion :exec
INSERT INTO compendium.effect_exhaustion (effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at)
VALUES (@effect_id, @ordinal, @d20_per_level, @speed_ft_per_level, @death_at);

-- name: ListEffectSaveEdges :many
SELECT effect_id, ordinal, ability, mode FROM compendium.effect_save_edges;

-- name: ListEffectCrits :many
SELECT effect_id, ordinal, feet FROM compendium.effect_crits;

-- name: ListEffectExhaustion :many
SELECT effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at FROM compendium.effect_exhaustion;

-- name: InsertEffectSpeedPenalty :exec
INSERT INTO compendium.effect_speed_penalties (effect_id, ordinal, ft) VALUES (@effect_id, @ordinal, @ft);

-- name: ListEffectSpeedPenalties :many
SELECT effect_id, ordinal, ft FROM compendium.effect_speed_penalties;

-- name: InsertEffectReaction :exec
INSERT INTO compendium.effect_reactions (effect_id, ordinal, trigger, instruction) VALUES (@effect_id, @ordinal, @trigger, @instruction);

-- name: ListEffectReactions :many
SELECT effect_id, ordinal, trigger, instruction FROM compendium.effect_reactions;

-- name: InsertEffectTempHP :exec
INSERT INTO compendium.effect_temp_hp (effect_id, ordinal, amount) VALUES (@effect_id, @ordinal, @amount);

-- name: ListEffectTempHPs :many
SELECT effect_id, ordinal, amount FROM compendium.effect_temp_hp;

-- name: InsertEffectTeleport :exec
INSERT INTO compendium.effect_teleports (effect_id, ordinal, range_ft) VALUES (@effect_id, @ordinal, @range_ft);

-- name: ListEffectTeleports :many
SELECT effect_id, ordinal, range_ft FROM compendium.effect_teleports;

-- name: InsertEffectForcedMove :exec
INSERT INTO compendium.effect_forced_moves (effect_id, ordinal, ft, toward) VALUES (@effect_id, @ordinal, @ft, @toward);

-- name: ListEffectForcedMoves :many
SELECT effect_id, ordinal, ft, toward FROM compendium.effect_forced_moves;

-- name: InsertEffectCounter :exec
INSERT INTO compendium.effect_counters (effect_id, ordinal, range_ft) VALUES (@effect_id, @ordinal, @range_ft);

-- name: ListEffectCounters :many
SELECT effect_id, ordinal, range_ft FROM compendium.effect_counters;

-- name: InsertEffectGrant :exec
INSERT INTO compendium.effect_grants (effect_id, ordinal, name) VALUES (@effect_id, @ordinal, @name);

-- name: ListEffectGrants :many
SELECT effect_id, ordinal, name FROM compendium.effect_grants;

-- name: InsertEffectResourceChange :exec
INSERT INTO compendium.effect_resource_changes (effect_id, ordinal, resource_slug, delta) VALUES (@effect_id, @ordinal, @resource_slug, @delta);

-- name: ListEffectResourceChanges :many
SELECT effect_id, ordinal, resource_slug, delta FROM compendium.effect_resource_changes;

-- name: InsertEffectMode :exec
INSERT INTO compendium.effect_modes (effect_id, ordinal, name) VALUES (@effect_id, @ordinal, @name);

-- name: ListEffectModes :many
SELECT effect_id, ordinal, name FROM compendium.effect_modes;

-- name: InsertEffectBranch :exec
INSERT INTO compendium.effect_branches (effect_id, ordinal, condition, n, creature_type) VALUES (@effect_id, @ordinal, @condition, @n, @creature_type);

-- name: ListEffectBranches :many
SELECT effect_id, ordinal, condition, n, creature_type FROM compendium.effect_branches;

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
