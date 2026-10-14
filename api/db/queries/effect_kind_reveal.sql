-- name: InsertEffectReveal :exec
INSERT INTO compendium.effect_reveals (effect_id, ordinal, qualities) VALUES (@effect_id, @ordinal, @qualities::text[]);

-- name: ListEffectReveals :many
SELECT effect_id, ordinal, qualities FROM compendium.effect_reveals;
