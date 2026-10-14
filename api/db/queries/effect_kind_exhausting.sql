-- name: InsertEffectExhaustion :exec
INSERT INTO compendium.effect_exhaustion (effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at, max_level)
VALUES (@effect_id, @ordinal, @d20_per_level, @speed_ft_per_level, @death_at, @max_level);

-- name: ListEffectExhaustion :many
SELECT effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at, max_level FROM compendium.effect_exhaustion;
