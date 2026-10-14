-- name: InsertEffectForm :exec
INSERT INTO compendium.effect_forms (effect_id, ordinal, monster_slug, temp_hp) VALUES (@effect_id, @ordinal, @monster_slug, @temp_hp);

-- name: ListEffectForms :many
SELECT effect_id, ordinal, monster_slug, temp_hp FROM compendium.effect_forms;
