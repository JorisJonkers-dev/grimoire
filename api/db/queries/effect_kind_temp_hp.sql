-- name: InsertEffectTempHP :exec
INSERT INTO compendium.effect_temp_hp (effect_id, ordinal, amount) VALUES (@effect_id, @ordinal, @amount);

-- name: ListEffectTempHPs :many
SELECT effect_id, ordinal, amount FROM compendium.effect_temp_hp;
