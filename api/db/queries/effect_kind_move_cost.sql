-- name: InsertEffectMoveCost :exec
INSERT INTO compendium.effect_move_costs (effect_id, ordinal, multiplier) VALUES (@effect_id, @ordinal, @multiplier);

-- name: ListEffectMoveCosts :many
SELECT effect_id, ordinal, multiplier FROM compendium.effect_move_costs;
