-- name: InsertEffectBranch :exec
INSERT INTO compendium.effect_branches (effect_id, ordinal, condition, n, creature_type) VALUES (@effect_id, @ordinal, @condition, @n, @creature_type);

-- name: ListEffectBranches :many
SELECT effect_id, ordinal, condition, n, creature_type FROM compendium.effect_branches;
