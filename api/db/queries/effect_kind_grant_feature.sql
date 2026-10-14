-- name: InsertEffectGrant :exec
INSERT INTO compendium.effect_grants (effect_id, ordinal, name) VALUES (@effect_id, @ordinal, @name);

-- name: ListEffectGrants :many
SELECT effect_id, ordinal, name FROM compendium.effect_grants;
