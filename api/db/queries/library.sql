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
SELECT c.id, c.name, l.pinned_revision FROM library.campaign_links l JOIN campaign.campaigns c ON c.id = l.campaign_id
WHERE l.entry_id = @entry_id ORDER BY lower(c.name), c.id;

-- name: LinkLibraryEntry :exec
INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES (@campaign_id, @entry_id, @now, @now)
ON CONFLICT (campaign_id, entry_id) DO NOTHING;

-- name: UnlinkLibraryEntry :execrows
DELETE FROM library.campaign_links WHERE campaign_id = @campaign_id AND entry_id = @entry_id;

-- name: SetLibraryOverride :execrows
UPDATE library.campaign_links SET override = @override, updated_at = @now WHERE campaign_id = @campaign_id AND entry_id = @entry_id;

-- name: PinLibraryRevision :execrows
UPDATE library.campaign_links SET pinned_revision = sqlc.narg(pinned_revision), updated_at = @now
WHERE campaign_id = @campaign_id AND entry_id = @entry_id;

-- name: CampaignLibraryLinks :many
SELECT e.id, e.owner_subject, e.kind, e.name, e.fields, e.revision, e.created_at, e.updated_at,
    l.pinned_revision, l.override, l.linked_at, r.name AS pinned_name, r.fields AS pinned_fields
FROM library.campaign_links l
JOIN library.entries e ON e.id = l.entry_id
LEFT JOIN library.entry_revisions r ON r.entry_id = l.entry_id AND r.no = l.pinned_revision
WHERE l.campaign_id = @campaign_id AND (sqlc.narg(entry_id)::uuid IS NULL OR l.entry_id = sqlc.narg(entry_id)::uuid)
ORDER BY e.kind, lower(e.name), e.id;
