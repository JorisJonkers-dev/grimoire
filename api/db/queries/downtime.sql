-- name: DowntimeCharacters :many
SELECT id, name, owner_member_id, downtime_days, downtime_spent FROM campaign.characters WHERE campaign_id = $1 ORDER BY name, id;

-- name: CampaignDowntime :one
SELECT game_day, game_minute, downtime_advanced FROM campaign.campaigns WHERE id = $1;

-- name: SetCharacterDowntime :exec
UPDATE campaign.characters SET downtime_days = @days, downtime_spent = @spent WHERE id = @id;

-- name: GrantDowntimeToAll :exec
-- A downtime given to the whole party is a new one: every Character starts it having lived through none of it.
UPDATE campaign.characters SET downtime_days = LEAST(downtime_days + @days, 100000), downtime_spent = 0 WHERE campaign_id = @campaign_id;

-- name: GrantDowntimeToOne :execrows
UPDATE campaign.characters SET downtime_days = LEAST(downtime_days + @days, 100000) WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetDowntimeClock :exec
UPDATE campaign.campaigns SET game_day = @game_day, game_minute = @game_minute, downtime_advanced = @advanced WHERE id = @id;

-- name: ListRecipes :many
SELECT id, campaign_id, name, item_slug, quantity, tool_slug, days, cost_cp, created_at FROM campaign.recipes WHERE campaign_id = $1 ORDER BY created_at, id;

-- name: ListRecipeIngredients :many
SELECT i.recipe_id, i.item_slug, i.count FROM campaign.recipe_ingredients i JOIN campaign.recipes r ON r.id = i.recipe_id
WHERE r.campaign_id = $1 ORDER BY i.recipe_id, i.position;

-- name: InsertRecipe :exec
INSERT INTO campaign.recipes (id, campaign_id, name, item_slug, quantity, tool_slug, days, cost_cp, created_at)
VALUES (@id, @campaign_id, @name, @item_slug, @quantity, sqlc.narg(tool_slug), @days, @cost_cp, @now);

-- name: InsertRecipeIngredient :exec
INSERT INTO campaign.recipe_ingredients (recipe_id, position, item_slug, count) VALUES (@recipe_id, @position, @item_slug, @count);

-- name: DeleteRecipe :execrows
DELETE FROM campaign.recipes WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertDowntimeLog :exec
INSERT INTO campaign.downtime_log (id, campaign_id, character_id, activity, detail, days, created_at)
VALUES (@id, @campaign_id, @character_id, @activity, @detail, @days, @now);

-- name: ListDowntimeLog :many
SELECT l.id, l.character_id, c.name AS character_name, l.activity, l.detail, l.days, l.created_at
FROM campaign.downtime_log l JOIN campaign.characters c ON c.id = l.character_id
WHERE l.campaign_id = $1 ORDER BY l.created_at DESC, l.id LIMIT 50;

-- name: TrainingDays :one
-- The days a Character has spent training in one thing.
SELECT coalesce(sum(days), 0)::int FROM campaign.downtime_log WHERE character_id = @character_id AND activity = 'train' AND detail = @detail;
