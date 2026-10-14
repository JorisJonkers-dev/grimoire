-- name: InsertEffectManual :exec
INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (@effect_id, @ordinal, @instruction);

-- name: ListEffectManual :many
SELECT effect_id, ordinal, instruction FROM compendium.effect_manual;
