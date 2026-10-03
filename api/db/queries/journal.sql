-- name: ListQuests :many
SELECT id, campaign_id, name, summary, status, created_at, updated_at FROM campaign.quests WHERE campaign_id = $1 ORDER BY created_at, id;

-- name: ListQuestSteps :many
SELECT s.quest_id, s.position, s.body, s.done
FROM campaign.quest_steps s JOIN campaign.quests q ON q.id = s.quest_id
WHERE q.campaign_id = $1 ORDER BY s.quest_id, s.position;

-- name: InsertQuest :exec
INSERT INTO campaign.quests (id, campaign_id, name, summary, status, created_at, updated_at)
VALUES (@id, @campaign_id, @name, @summary, @status, @now, @now);

-- name: UpdateQuest :execrows
UPDATE campaign.quests SET name = @name, summary = @summary, status = @status, updated_at = @now WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteQuest :execrows
DELETE FROM campaign.quests WHERE campaign_id = @campaign_id AND id = @id;

-- name: ClearQuestSteps :exec
DELETE FROM campaign.quest_steps WHERE quest_id = $1;

-- name: InsertQuestStep :exec
INSERT INTO campaign.quest_steps (quest_id, position, body, done) VALUES (@quest_id, @position, @body, @done);

-- name: ListLore :many
SELECT id, campaign_id, title, body, item_slug, unlocked_at, created_at, updated_at FROM campaign.lore WHERE campaign_id = $1 ORDER BY title, id;

-- name: InsertLore :exec
INSERT INTO campaign.lore (id, campaign_id, title, body, item_slug, unlocked_at, created_at, updated_at)
VALUES (@id, @campaign_id, @title, @body, @item_slug, sqlc.narg(unlocked_at), @now, @now);

-- name: UpdateLore :execrows
UPDATE campaign.lore SET title = @title, body = @body, item_slug = @item_slug, unlocked_at = sqlc.narg(unlocked_at), updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteLore :execrows
DELETE FROM campaign.lore WHERE campaign_id = @campaign_id AND id = @id;

-- name: UnlockLoreByItem :many
-- Reading an item unlocks every Lore entry of the Campaign it holds that is still locked.
UPDATE campaign.lore SET unlocked_at = @now, updated_at = @now
WHERE campaign_id = @campaign_id AND item_slug = @item_slug AND item_slug <> '' AND unlocked_at IS NULL
RETURNING id;

-- name: CarriedItems :many
-- The items a Member can read: what their own Characters carry in the Campaign and what lies in its
-- Party Stash, in the bags inside those too.
WITH RECURSIVE held AS (
    SELECT c.id
    FROM campaign.containers c
    LEFT JOIN campaign.characters ch ON ch.id = c.character_id
    WHERE c.campaign_id = @campaign_id AND (c.kind = 'party_stash' OR ch.owner_member_id = @member_id)
    UNION
    SELECT b.id
    FROM campaign.containers b
    JOIN held h ON b.parent_id = h.id
)
SELECT DISTINCT i.item_slug
FROM campaign.item_instances i
JOIN held h ON h.id = i.container_id
ORDER BY i.item_slug;
