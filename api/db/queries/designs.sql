-- name: LatestDesignHash :one
SELECT coalesce((SELECT design_hash FROM ops.design_loads ORDER BY id DESC LIMIT 1), '')::text AS design_hash;

-- name: RecordDesignLoad :exec
INSERT INTO ops.design_loads (design_hash) VALUES (@design_hash);

-- name: DeleteDesignEffects :exec
DELETE FROM compendium.effect_definitions d
USING compendium.design_effects o
WHERE o.effect_id = d.id AND NOT (d.slug = ANY (@keep::text[]));

-- name: ClearDesignEffects :exec
DELETE FROM compendium.design_effects;

-- name: MarkDesignEffect :exec
INSERT INTO compendium.design_effects (effect_id)
SELECT id FROM compendium.effect_definitions WHERE slug = @slug;
