-- name: InsertEffectExtraDamage :exec
INSERT INTO compendium.effect_extra_damage (effect_id, ordinal, dice) VALUES (@effect_id, @ordinal, @dice);

-- name: ListEffectExtraDamage :many
SELECT effect_id, ordinal, dice FROM compendium.effect_extra_damage;
