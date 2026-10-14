-- name: InsertEffectEdge :exec
INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach, source_only)
VALUES (@effect_id, @ordinal, @against, @advantage, @reach, @source_only);

-- name: ListEffectEdges :many
SELECT effect_id, ordinal, against, advantage, reach, source_only FROM compendium.effect_edges;
