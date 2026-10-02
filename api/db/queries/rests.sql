-- name: GetRest :one
SELECT kind, status, proposed_by, dm_agreed FROM play.rests WHERE session_id = $1;

-- name: ListRestAgreements :many
SELECT member_id FROM play.rest_agreements WHERE session_id = $1 ORDER BY member_id;

-- name: ListRestResters :many
SELECT character_id, token_id, roll_id FROM play.rest_resters WHERE session_id = $1 ORDER BY character_id;

-- name: SaveRest :exec
INSERT INTO play.rests (session_id, kind, status, proposed_by, dm_agreed) VALUES (@session_id, @kind, @status, @proposed_by, @dm_agreed)
ON CONFLICT (session_id) DO UPDATE SET kind = excluded.kind, status = excluded.status, proposed_by = excluded.proposed_by, dm_agreed = excluded.dm_agreed;

-- name: ClearRest :exec
DELETE FROM play.rests WHERE session_id = $1;

-- name: ClearRestMembers :exec
WITH gone AS (DELETE FROM play.rest_agreements a WHERE a.session_id = @session_id)
DELETE FROM play.rest_resters r WHERE r.session_id = @session_id;

-- name: AddRestAgreement :exec
INSERT INTO play.rest_agreements (session_id, member_id) VALUES (@session_id, @member_id);

-- name: AddRestRester :exec
INSERT INTO play.rest_resters (session_id, character_id, token_id, roll_id) VALUES (@session_id, @character_id, @token_id, sqlc.narg(roll_id));

-- name: RestCharacters :many
SELECT c.id, c.name, c.class_slug, c.level, c.hit_dice_spent,
       COALESCE((SELECT k.hit_die FROM compendium.classes k JOIN compendium.documents d ON d.id = k.document_id
                 WHERE k.slug = c.class_slug ORDER BY d.precedence DESC LIMIT 1), 8)::integer AS hit_die
FROM campaign.characters c WHERE c.campaign_id = @campaign_id AND c.id = ANY(@ids::uuid[]) ORDER BY c.name, c.id;

-- name: RestAbilities :many
SELECT character_id, ability, base + bonus AS score FROM campaign.character_abilities WHERE character_id = ANY(@ids::uuid[]);

-- name: RestResourcesUsed :many
SELECT character_id, resource_slug, used FROM campaign.character_resources WHERE character_id = ANY(@ids::uuid[]);

-- name: CampaignRestSupplies :one
SELECT rest_supplies FROM campaign.campaigns WHERE id = $1;

-- name: SpendHitDie :exec
UPDATE campaign.characters SET hit_dice_spent = hit_dice_spent + 1 WHERE id = $1;

-- name: SaveRestResult :exec
UPDATE campaign.characters
SET hp_current = LEAST(hp_max, GREATEST(0, @hp_current::integer)), hit_dice_spent = @hit_dice_spent, level_up_ready = level_up_ready OR @level_up_ready
WHERE id = @id;

-- name: SetResourceUsed :exec
INSERT INTO campaign.character_resources (character_id, resource_slug, used) VALUES (@character_id, @resource_slug, @used)
ON CONFLICT (character_id, resource_slug) DO UPDATE SET used = excluded.used;

-- name: ChangeResourceUsed :exec
INSERT INTO campaign.character_resources (character_id, resource_slug, used)
SELECT c.id, @resource_slug::text, LEAST(100, GREATEST(0, 0 - @delta::integer)) FROM campaign.characters c WHERE c.id = @character_id
ON CONFLICT (character_id, resource_slug) DO UPDATE SET used = LEAST(100, GREATEST(0, campaign.character_resources.used - @delta::integer));

-- name: NotifyLevelUp :exec
-- The Account playing a Campaign Character hears it may level up, unless it turned that off in app.
INSERT INTO social.notifications (id, account_id, kind, title, body, action_label, action_path, dedupe_key, created_at)
SELECT gen_random_uuid(), a.id, 'level_up', left(c.name || ' can level up', 120), 'A long rest unlocked the next level.', 'Open',
    '/campaigns/' || c.campaign_id || '/characters/' || c.id, 'level_up:' || c.id, @now
FROM campaign.characters c
JOIN campaign.account_characters o ON o.id = c.character_id
JOIN identity.accounts a ON a.subject = o.owner_subject
WHERE c.id = @character_id
    AND coalesce((SELECT p.enabled FROM social.notification_preferences p WHERE p.account_id = a.id AND p.kind = 'level_up' AND p.channel = 'in_app'), true)
ON CONFLICT (account_id, dedupe_key) WHERE read_at IS NULL AND dedupe_key <> '' DO UPDATE SET created_at = EXCLUDED.created_at;
