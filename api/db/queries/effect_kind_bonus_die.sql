-- name: InsertEffectBonusDie :exec
INSERT INTO compendium.effect_bonus_dice (effect_id, ordinal, dice, on_attacks, on_saves)
VALUES (@effect_id, @ordinal, @dice, @on_attacks, @on_saves);

-- name: ListEffectBonusDice :many
SELECT effect_id, ordinal, dice, on_attacks, on_saves FROM compendium.effect_bonus_dice;
