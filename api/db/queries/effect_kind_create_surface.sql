-- name: InsertEffectSurface :exec
INSERT INTO compendium.effect_surfaces (effect_id, ordinal, surface, rounds) VALUES (@effect_id, @ordinal, @surface, @rounds);

-- name: ListEffectSurfaces :many
SELECT effect_id, ordinal, surface, rounds FROM compendium.effect_surfaces;
