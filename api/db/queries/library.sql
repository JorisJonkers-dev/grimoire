-- name: InsertLibraryEntry :exec
INSERT INTO library.entries (id, owner_subject, kind, name, fields, revision, created_at, updated_at)
VALUES (@id, @owner_subject, @kind, @name, @fields, 1, @now, @now);

-- name: UpdateLibraryEntry :one
UPDATE library.entries SET name = @name, fields = @fields, revision = revision + 1, updated_at = @now WHERE id = @id RETURNING revision;

-- name: InsertLibraryRevision :exec
INSERT INTO library.entry_revisions (entry_id, no, name, fields, author_subject, created_at)
VALUES (@entry_id, @no, @name, @fields, @author_subject, @now);

-- name: LibraryEntry :one
SELECT id, owner_subject, kind, name, fields, revision, created_at, updated_at FROM library.entries WHERE id = @id;

-- name: LibraryEntries :many
SELECT id, owner_subject, kind, name, fields, revision, created_at, updated_at FROM library.entries
WHERE owner_subject = @owner_subject AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
ORDER BY kind, lower(name), id;

-- name: LibraryRevisions :many
SELECT no, name, fields, author_subject, created_at FROM library.entry_revisions WHERE entry_id = @entry_id ORDER BY no DESC;

-- name: LibraryRevisionExists :one
SELECT EXISTS (SELECT 1 FROM library.entry_revisions WHERE entry_id = @entry_id AND no = @no);

-- name: LibraryEntryUses :many
SELECT c.id, c.name, l.pinned_revision FROM campaign.campaigns c
LEFT JOIN library.campaign_links l ON l.campaign_id = c.id AND l.entry_id = @entry_id
WHERE coalesce(l.direct, false) OR EXISTS (
    SELECT 1 FROM library.collection_entries ce JOIN library.campaign_collections cc ON cc.collection_id = ce.collection_id
    WHERE ce.entry_id = @entry_id AND cc.campaign_id = c.id)
ORDER BY lower(c.name), c.id;

-- name: LinkLibraryEntry :exec
INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES (@campaign_id, @entry_id, @now, @now)
ON CONFLICT (campaign_id, entry_id) DO UPDATE SET direct = true, updated_at = excluded.updated_at;

-- name: UnlinkLibraryEntry :execrows
DELETE FROM library.campaign_links WHERE campaign_id = @campaign_id AND entry_id = @entry_id AND direct;

-- name: SetLibraryOverride :exec
INSERT INTO library.campaign_links (campaign_id, entry_id, override, direct, linked_at, updated_at)
VALUES (@campaign_id, @entry_id, @override, false, @now, @now)
ON CONFLICT (campaign_id, entry_id) DO UPDATE SET override = excluded.override, updated_at = excluded.updated_at;

-- name: PinLibraryRevision :exec
INSERT INTO library.campaign_links (campaign_id, entry_id, pinned_revision, direct, linked_at, updated_at)
VALUES (@campaign_id, @entry_id, sqlc.narg(pinned_revision), false, @now, @now)
ON CONFLICT (campaign_id, entry_id) DO UPDATE SET pinned_revision = excluded.pinned_revision, updated_at = excluded.updated_at;

-- name: CampaignLibraryLinks :many
WITH visible AS (
    SELECT l.entry_id FROM library.campaign_links l WHERE l.campaign_id = @campaign_id AND l.direct
    UNION
    SELECT ce.entry_id FROM library.collection_entries ce
    JOIN library.campaign_collections cc ON cc.collection_id = ce.collection_id WHERE cc.campaign_id = @campaign_id
)
SELECT e.id, e.owner_subject, e.kind, e.name, e.fields, e.revision, e.created_at, e.updated_at,
    l.pinned_revision, coalesce(l.override, '{}'::jsonb)::jsonb AS override, coalesce(l.direct, false)::boolean AS direct,
    r.name AS pinned_name, r.fields AS pinned_fields,
    ARRAY(SELECT c.name FROM library.collection_entries ce
        JOIN library.campaign_collections cc ON cc.collection_id = ce.collection_id AND cc.campaign_id = @campaign_id
        JOIN library.collections c ON c.id = ce.collection_id
        WHERE ce.entry_id = e.id ORDER BY c.name)::text[] AS via
FROM visible v
JOIN library.entries e ON e.id = v.entry_id
LEFT JOIN library.campaign_links l ON l.entry_id = e.id AND l.campaign_id = @campaign_id
LEFT JOIN library.entry_revisions r ON r.entry_id = e.id AND r.no = l.pinned_revision
WHERE sqlc.narg(entry_id)::uuid IS NULL OR e.id = sqlc.narg(entry_id)::uuid
ORDER BY e.kind, lower(e.name), e.id;

-- name: InsertLibraryCollection :exec
INSERT INTO library.collections (id, owner_subject, name, description, created_at, updated_at)
VALUES (@id, @owner_subject, @name, @description, @now, @now);

-- name: UpdateLibraryCollection :exec
UPDATE library.collections SET name = @name, description = @description, updated_at = @now WHERE id = @id;

-- name: ClearLibraryCollection :exec
DELETE FROM library.collection_entries WHERE collection_id = @collection_id;

-- name: AddToLibraryCollection :exec
INSERT INTO library.collection_entries (collection_id, entry_id) VALUES (@collection_id, @entry_id) ON CONFLICT DO NOTHING;

-- name: LibraryCollection :one
SELECT c.id, c.owner_subject, c.name, c.description, c.created_at, c.updated_at,
    ARRAY(SELECT ce.entry_id FROM library.collection_entries ce WHERE ce.collection_id = c.id ORDER BY ce.entry_id)::uuid[] AS entry_ids
FROM library.collections c WHERE c.id = @id;

-- name: CampaignLibraryCollections :many
SELECT c.id, c.owner_subject, c.name, c.description, c.created_at, c.updated_at,
    ARRAY(SELECT ce.entry_id FROM library.collection_entries ce WHERE ce.collection_id = c.id ORDER BY ce.entry_id)::uuid[] AS entry_ids,
    (cc.campaign_id IS NOT NULL)::boolean AS switched_on
FROM library.collections c
LEFT JOIN library.campaign_collections cc ON cc.collection_id = c.id AND cc.campaign_id = sqlc.narg(campaign_id)::uuid
WHERE c.owner_subject = @owner_subject OR cc.campaign_id IS NOT NULL
ORDER BY lower(c.name), c.id;

-- name: SwitchOnLibraryCollection :exec
INSERT INTO library.campaign_collections (campaign_id, collection_id, switched_at) VALUES (@campaign_id, @collection_id, @now)
ON CONFLICT DO NOTHING;

-- name: SwitchOffLibraryCollection :exec
DELETE FROM library.campaign_collections WHERE campaign_id = @campaign_id AND collection_id = @collection_id;
