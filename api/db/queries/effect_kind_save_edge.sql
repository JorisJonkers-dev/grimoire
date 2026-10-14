-- name: InsertEffectSaveEdge :exec
INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (@effect_id, @ordinal, @ability, @mode);

-- name: ListEffectSaveEdges :many
SELECT effect_id, ordinal, ability, mode FROM compendium.effect_save_edges;
