-- name: InsertEffectCrit :exec
INSERT INTO compendium.effect_crits (effect_id, ordinal, feet) VALUES (@effect_id, @ordinal, @feet);

-- name: ListEffectCrits :many
SELECT effect_id, ordinal, feet FROM compendium.effect_crits;
