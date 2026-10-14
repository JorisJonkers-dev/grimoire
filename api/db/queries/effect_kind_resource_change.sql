-- name: InsertEffectResourceChange :exec
INSERT INTO compendium.effect_resource_changes (effect_id, ordinal, resource_slug, delta) VALUES (@effect_id, @ordinal, @resource_slug, @delta);

-- name: ListEffectResourceChanges :many
SELECT effect_id, ordinal, resource_slug, delta FROM compendium.effect_resource_changes;
