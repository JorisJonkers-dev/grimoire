-- name: InsertEffectSummon :exec
INSERT INTO compendium.effect_summons (effect_id, ordinal, monster_slug, count, shares_turn, needs_command)
VALUES (@effect_id, @ordinal, @monster_slug, @count, @shares_turn, @needs_command);

-- name: ListEffectSummons :many
SELECT effect_id, ordinal, monster_slug, count, shares_turn, needs_command FROM compendium.effect_summons;
