-- name: InsertEffectSpeedPenalty :exec
INSERT INTO compendium.effect_speed_penalties (effect_id, ordinal, ft) VALUES (@effect_id, @ordinal, @ft);

-- name: ListEffectSpeedPenalties :many
SELECT effect_id, ordinal, ft FROM compendium.effect_speed_penalties;
