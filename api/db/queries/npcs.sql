-- name: InsertNPC :one
INSERT INTO campaign.npcs (id, campaign_id, name, title, description, dm_notes, disposition, updated_at)
VALUES (coalesce(sqlc.narg(id)::uuid, gen_random_uuid()), @campaign_id, @name, @title, @description, @dm_notes, @disposition, @now)
RETURNING id;

-- name: UpdateNPC :execrows
UPDATE campaign.npcs SET name = @name, title = @title, description = @description, dm_notes = @dm_notes,
    disposition = @disposition, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteNPC :execrows
DELETE FROM campaign.npcs WHERE campaign_id = @campaign_id AND id = @id;

-- name: GetNPC :one
SELECT id, name, title, description, dm_notes, disposition, updated_at
FROM campaign.npcs WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListNPCs :many
SELECT id, name, title, description, dm_notes, disposition, updated_at
FROM campaign.npcs WHERE campaign_id = $1 ORDER BY name, id;

-- name: NextRevisionNo :one
SELECT coalesce(max(revision_no), 0)::int + 1 FROM campaign.revisions WHERE entity_type = @entity_type AND entity_id = @entity_id;

-- name: InsertRevision :one
INSERT INTO campaign.revisions (campaign_id, entity_type, entity_id, revision_no, action, caller_subject, caller_name, origin, client,
    restored_from, created_at)
VALUES (@campaign_id, @entity_type, @entity_id, @revision_no, @action, @caller_subject, @caller_name, @origin, @client,
    sqlc.narg(restored_from), @now)
RETURNING id;

-- name: InsertNPCRevision :exec
INSERT INTO campaign.npc_revisions (revision_id, name, title, description, dm_notes, disposition)
VALUES (@revision_id, @name, @title, @description, @dm_notes, @disposition);

-- name: ListRevisions :many
SELECT revision_no, action, caller_name, origin, client, restored_from, created_at
FROM campaign.revisions
WHERE campaign_id = @campaign_id AND entity_type = @entity_type AND entity_id = @entity_id
ORDER BY revision_no DESC;

-- name: GetNPCRevision :one
SELECT r.revision_no, r.action, n.name, n.title, n.description, n.dm_notes, n.disposition
FROM campaign.revisions r JOIN campaign.npc_revisions n ON n.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'npc' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: DeletedNPCs :many
SELECT DISTINCT ON (r.entity_id) r.entity_id, n.name, r.created_at
FROM campaign.revisions r JOIN campaign.npc_revisions n ON n.revision_id = r.id
WHERE r.campaign_id = $1 AND r.entity_type = 'npc'
  AND NOT EXISTS (SELECT 1 FROM campaign.npcs x WHERE x.id = r.entity_id)
ORDER BY r.entity_id, r.revision_no DESC;

-- name: LockEntity :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));
