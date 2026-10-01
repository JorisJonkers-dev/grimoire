-- name: ListResources :many
SELECT id, slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level FROM compendium.resources ORDER BY slug;

-- name: ListResourceMaxima :many
SELECT resource_id, level, maximum FROM compendium.resource_maxima ORDER BY resource_id, level;

-- name: ListResourceDice :many
SELECT resource_id, level, die FROM compendium.resource_dice ORDER BY resource_id, level;

-- name: ListResourceRecharges :many
SELECT resource_id, event, from_level, amount, roll_at_least FROM compendium.resource_recharges ORDER BY resource_id, event, from_level;

-- name: ListScales :many
SELECT id, slug, name, owner_kind, owner_slug FROM compendium.scales ORDER BY slug;

-- name: ListScaleSteps :many
SELECT scale_id, level, value FROM compendium.scale_steps ORDER BY scale_id, level;

-- name: ListChoices :many
SELECT owner_kind, owner_slug, slug, name, level, count, pool, pool_from FROM compendium.choices ORDER BY owner_kind, owner_slug, level, slug;

-- name: ListPrerequisites :many
SELECT owner_kind, owner_slug, group_no, ordinal, kind, ability, minimum, ref_slug FROM compendium.prerequisites
ORDER BY owner_kind, owner_slug, group_no, ordinal;
