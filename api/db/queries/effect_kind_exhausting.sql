-- name: InsertEffectExhaustion :exec
INSERT INTO compendium.effect_exhaustion (effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at)
VALUES (@effect_id, @ordinal, @d20_per_level, @speed_ft_per_level, @death_at);

-- name: ListEffectExhaustion :many
SELECT effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at FROM compendium.effect_exhaustion;
