-- name: InsertEffectAreaSave :exec
INSERT INTO compendium.effect_area_saves (effect_id, ordinal, ability) VALUES (@effect_id, @ordinal, @ability);

-- name: ListEffectAreaSaves :many
SELECT effect_id, ordinal, ability FROM compendium.effect_area_saves;
