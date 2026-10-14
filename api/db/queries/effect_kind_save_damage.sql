-- name: InsertEffectSaveDamage :exec
INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half)
VALUES (@effect_id, @ordinal, @ability, @dice, @damage_type, @half);

-- name: ListEffectSaveDamage :many
SELECT effect_id, ordinal, ability, dice, damage_type, half FROM compendium.effect_save_damage;
