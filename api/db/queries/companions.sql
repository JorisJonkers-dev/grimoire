-- name: ListCompanions :many
SELECT id, campaign_id, name, kind, monster_slug, controller_member_id, shares_xp, hp_current, notes, created_at, updated_at
FROM campaign.companions WHERE campaign_id = @campaign_id ORDER BY lower(name), id;

-- name: GetCompanion :one
SELECT id, campaign_id, name, kind, monster_slug, controller_member_id, shares_xp, hp_current, notes, created_at, updated_at
FROM campaign.companions WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertCompanion :exec
INSERT INTO campaign.companions (id, campaign_id, name, kind, monster_slug, controller_member_id, shares_xp, notes, created_at, updated_at)
VALUES (@id, @campaign_id, @name, @kind, @monster_slug, sqlc.narg(controller_member_id), @shares_xp, @notes, @now, @now);

-- name: UpdateCompanion :execrows
-- A Companion whose creature changes starts again at that creature's full hit points.
UPDATE campaign.companions SET name = @name, kind = @kind,
    hp_current = CASE WHEN monster_slug = @monster_slug THEN hp_current END, monster_slug = @monster_slug,
    controller_member_id = sqlc.narg(controller_member_id), shares_xp = @shares_xp, notes = @notes, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteCompanion :execrows
DELETE FROM campaign.companions WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetCompanionController :exec
-- Only a Companion of the Campaign the Session is played in changes hands there.
UPDATE campaign.companions c SET controller_member_id = sqlc.narg(controller_member_id)
FROM play.sessions s WHERE c.id = @id AND s.id = @session_id AND c.campaign_id = s.campaign_id;

-- name: KeepCompanionHP :exec
-- A Companion keeps the hit points its token had when it left the map: one of the Session's own Campaign only.
UPDATE campaign.companions c SET hp_current = t.hp
FROM play.tokens t, play.sessions s
WHERE t.session_id = @session_id AND t.id = @token_id AND t.companion_id = c.id AND t.hp IS NOT NULL
  AND s.id = t.session_id AND c.campaign_id = s.campaign_id;

-- name: KeepCompanionsHP :exec
-- And those of every token still on the map when the Session ends.
UPDATE campaign.companions c SET hp_current = t.hp
FROM play.tokens t, play.sessions s
WHERE t.session_id = @session_id AND t.companion_id = c.id AND t.hp IS NOT NULL
  AND s.id = t.session_id AND c.campaign_id = s.campaign_id;

-- name: SharingCompanions :one
-- How many of these Companions take a share of the XP.
SELECT count(*)::int FROM campaign.companions WHERE campaign_id = @campaign_id AND shares_xp AND id = ANY(@ids::uuid[]);

-- name: SetTokenController :exec
UPDATE play.tokens SET controller_member_id = sqlc.narg(controller_member_id) WHERE session_id = @session_id AND id = @id;

-- name: AwardXP :exec
UPDATE campaign.characters SET xp = xp + @amount WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertXPAward :exec
-- Recorded for a Character the Campaign still has; a token whose Character is gone earns nothing.
INSERT INTO play.xp_awards (action_id, character_id, amount)
SELECT @action_id, c.id, @amount FROM campaign.characters c WHERE c.id = @character_id AND c.campaign_id = @campaign_id;
