-- name: InsertEffectLight :exec
INSERT INTO compendium.effect_lights (effect_id, ordinal, bright_ft, dim_ft) VALUES (@effect_id, @ordinal, @bright_ft, @dim_ft);

-- name: ListEffectLights :many
SELECT effect_id, ordinal, bright_ft, dim_ft FROM compendium.effect_lights;
