-- name: InsertEffectReaction :exec
INSERT INTO compendium.effect_reactions (effect_id, ordinal, trigger, instruction) VALUES (@effect_id, @ordinal, @trigger, @instruction);

-- name: ListEffectReactions :many
SELECT effect_id, ordinal, trigger, instruction FROM compendium.effect_reactions;
