-- name: InsertEffectArea :exec
INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft)
VALUES (@effect_id, @ordinal, @shape, @size_ft, @range_ft);

-- name: ListEffectAreas :many
SELECT effect_id, ordinal, shape, size_ft, range_ft FROM compendium.effect_areas;
