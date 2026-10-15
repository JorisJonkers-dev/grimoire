-- name: UpsertEffectDefinition :one
INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount, repeat_save)
VALUES (@slug, @name, @concentration, @owner_kind, @owner_slug, sqlc.narg(duration_kind), @duration_amount, sqlc.narg(repeat_save))
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, concentration = EXCLUDED.concentration,
    owner_kind = EXCLUDED.owner_kind, owner_slug = EXCLUDED.owner_slug,
    duration_kind = EXCLUDED.duration_kind, duration_amount = EXCLUDED.duration_amount, repeat_save = EXCLUDED.repeat_save
RETURNING id;

-- name: ClearEffectParts :exec
DELETE FROM compendium.effect_components WHERE effect_id = @effect_id;

-- name: InsertEffectPart :exec
INSERT INTO compendium.effect_components (effect_id, ordinal, kind, parent, group_name)
VALUES (@effect_id, @ordinal, @kind, sqlc.narg(parent), sqlc.narg(group_name));

-- name: ListEffectDefinitions :many
SELECT id, slug, name, concentration, owner_kind, duration_kind, duration_amount, repeat_save FROM compendium.effect_definitions ORDER BY slug;

-- name: ListEffectParts :many
SELECT effect_id, ordinal, kind, parent, group_name FROM compendium.effect_components ORDER BY effect_id, ordinal;

-- name: ListEffectPartsOfKind :many
SELECT effect_id, ordinal FROM compendium.effect_components WHERE kind = @kind;

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
