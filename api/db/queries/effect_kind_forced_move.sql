-- name: InsertEffectForcedMove :exec
INSERT INTO compendium.effect_forced_moves (effect_id, ordinal, ft, toward) VALUES (@effect_id, @ordinal, @ft, @toward);

-- name: ListEffectForcedMoves :many
SELECT effect_id, ordinal, ft, toward FROM compendium.effect_forced_moves;
