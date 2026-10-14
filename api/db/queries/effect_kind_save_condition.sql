-- name: InsertEffectSaveCondition :exec
INSERT INTO compendium.effect_save_conditions (effect_id, ordinal, ability, condition_slug)
VALUES (@effect_id, @ordinal, @ability, @condition_slug);

-- name: ListEffectSaveConditions :many
SELECT effect_id, ordinal, ability, condition_slug FROM compendium.effect_save_conditions;
