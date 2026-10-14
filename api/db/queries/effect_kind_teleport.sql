-- name: InsertEffectTeleport :exec
INSERT INTO compendium.effect_teleports (effect_id, ordinal, range_ft) VALUES (@effect_id, @ordinal, @range_ft);

-- name: ListEffectTeleports :many
SELECT effect_id, ordinal, range_ft FROM compendium.effect_teleports;
