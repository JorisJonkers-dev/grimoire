-- name: InsertEffectCounter :exec
INSERT INTO compendium.effect_counters (effect_id, ordinal, range_ft) VALUES (@effect_id, @ordinal, @range_ft);

-- name: ListEffectCounters :many
SELECT effect_id, ordinal, range_ft FROM compendium.effect_counters;
