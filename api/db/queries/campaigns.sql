-- name: CreateCampaign :one
INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at)
VALUES (@name, @ruleset_pref, @created_by, @now, @now)
RETURNING id, name, ruleset_pref, reaction_timeout_s, high_ground, rest_supplies, initiative_mode, share_initiative, created_at;

-- name: UpdateCampaign :one
UPDATE campaign.campaigns
SET name = coalesce(sqlc.narg(name)::text, name),
    ruleset_pref = coalesce(sqlc.narg(ruleset_pref)::text, ruleset_pref),
    reaction_timeout_s = coalesce(sqlc.narg(reaction_timeout_s)::integer, reaction_timeout_s),
    high_ground = coalesce(sqlc.narg(high_ground)::boolean, high_ground),
    rest_supplies = coalesce(sqlc.narg(rest_supplies)::boolean, rest_supplies),
    initiative_mode = coalesce(sqlc.narg(initiative_mode)::text, initiative_mode),
    share_initiative = coalesce(sqlc.narg(share_initiative)::boolean, share_initiative),
    updated_at = @now
WHERE id = @id
RETURNING id, name, ruleset_pref, reaction_timeout_s, high_ground, rest_supplies, initiative_mode, share_initiative, created_at;

-- name: GetCampaign :one
SELECT id, name, ruleset_pref, reaction_timeout_s, high_ground, rest_supplies, initiative_mode, share_initiative, created_at FROM campaign.campaigns WHERE id = $1;

-- name: ListCampaignsForSubject :many
SELECT c.id, c.name, c.ruleset_pref, c.reaction_timeout_s, c.high_ground, c.rest_supplies, c.initiative_mode, c.share_initiative, c.created_at, m.role,
       (SELECT count(*) FROM campaign.members x WHERE x.campaign_id = c.id)::int AS member_count
FROM campaign.campaigns c
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject
WHERE sqlc.narg(after_created)::timestamptz IS NULL
   OR (c.created_at, c.id) < (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::uuid)
ORDER BY c.created_at DESC, c.id DESC
LIMIT @page_size;

-- name: AddMember :one
INSERT INTO campaign.members (campaign_id, auth_subject, display_name, role, joined_at)
VALUES (@campaign_id, @auth_subject, @display_name, @role, @now)
ON CONFLICT (campaign_id, auth_subject) DO UPDATE SET display_name = campaign.members.display_name
RETURNING id, campaign_id, auth_subject, display_name, role, joined_at;

-- name: GetMembership :one
SELECT id, campaign_id, auth_subject, display_name, role, joined_at
FROM campaign.members WHERE campaign_id = @campaign_id AND auth_subject = @auth_subject;

-- name: GetMember :one
SELECT id, campaign_id, auth_subject, display_name, role, joined_at
FROM campaign.members WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListMembers :many
SELECT id, campaign_id, auth_subject, display_name, role, joined_at
FROM campaign.members WHERE campaign_id = $1 ORDER BY role, display_name, id;

-- name: SetMemberRole :exec
UPDATE campaign.members SET role = @role WHERE campaign_id = @campaign_id AND id = @id;

-- name: RemoveMember :exec
DELETE FROM campaign.members WHERE campaign_id = @campaign_id AND id = @id;

-- name: CountDMs :one
SELECT count(*)::int FROM campaign.members WHERE campaign_id = $1 AND role = 'dm';

-- name: LockCampaign :exec
SELECT id FROM campaign.campaigns WHERE id = $1 FOR UPDATE;

-- name: CreateInvite :one
INSERT INTO campaign.invites (campaign_id, token_hash, created_by, created_at, expires_at)
VALUES (@campaign_id, @token_hash, @created_by, @now, @expires_at)
RETURNING id, created_at, expires_at;

-- name: ListInvites :many
SELECT i.id, i.created_at, i.expires_at, m.display_name AS created_by_name
FROM campaign.invites i JOIN campaign.members m ON m.id = i.created_by
WHERE i.campaign_id = @campaign_id AND i.revoked_at IS NULL AND i.expires_at > @now
ORDER BY i.created_at DESC, i.id DESC;

-- name: RevokeInvite :execrows
UPDATE campaign.invites SET revoked_at = @now::timestamptz
WHERE campaign_id = @campaign_id AND id = @id AND revoked_at IS NULL;

-- name: FindInvite :one
SELECT i.id, i.campaign_id, c.name AS campaign_name, m.display_name AS created_by_name
FROM campaign.invites i
JOIN campaign.campaigns c ON c.id = i.campaign_id
JOIN campaign.members m ON m.id = i.created_by
WHERE i.token_hash = @token_hash AND i.revoked_at IS NULL AND i.expires_at > @now;
