-- name: ClearEffectScalingRules :exec
DELETE FROM compendium.effect_scaling_rules WHERE effect_id = @effect_id;

-- name: InsertEffectScalingRule :exec
INSERT INTO compendium.effect_scaling_rules (effect_id, position, part, field, axis, class_slug, rule, base_level, per_dice, per_number, formula, column_name)
VALUES (@effect_id, @position, sqlc.narg(part), @field, @axis, @class_slug, @rule, @base_level, sqlc.narg(per_dice), sqlc.narg(per_number), sqlc.narg(formula), @column_name);

-- name: InsertEffectScalingRuleStep :exec
INSERT INTO compendium.effect_scaling_rule_steps (effect_id, position, at_level, value_dice, value_number, value_flag, value_text)
VALUES (@effect_id, @position, @at_level, sqlc.narg(value_dice), sqlc.narg(value_number), sqlc.narg(value_flag), sqlc.narg(value_text));

-- name: ListEffectScalingRules :many
SELECT effect_id, position, part, field, axis, class_slug, rule, base_level, per_dice, per_number, formula, column_name
FROM compendium.effect_scaling_rules ORDER BY effect_id, position;

-- name: ListEffectScalingRuleSteps :many
SELECT effect_id, position, at_level, value_dice, value_number, value_flag, value_text
FROM compendium.effect_scaling_rule_steps ORDER BY effect_id, position, at_level;
