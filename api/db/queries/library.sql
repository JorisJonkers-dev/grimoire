-- name: InsertLibraryEntry :exec
INSERT INTO library.entries (id, owner_subject, kind, name, fields, revision, created_at, updated_at, shared, design)
VALUES (@id, @owner_subject, @kind, @name, @fields, 1, @now, @now, @shared, sqlc.narg(design));

-- name: UpdateLibraryEntry :one
UPDATE library.entries SET name = @name, fields = @fields, design = sqlc.narg(design), revision = revision + 1, updated_at = @now
WHERE id = @id RETURNING revision;

-- name: InsertLibraryRevision :exec
INSERT INTO library.entry_revisions (entry_id, no, name, fields, author_subject, created_at, design, origin, client)
VALUES (@entry_id, @no, @name, @fields, @author_subject, @now, sqlc.narg(design), @origin, sqlc.narg(client));

-- name: LibraryEntry :one
SELECT id, owner_subject, kind, name, fields, revision, created_at, updated_at, shared, design FROM library.entries WHERE id = @id;

-- name: LibraryEntries :many
SELECT id, owner_subject, kind, name, fields, revision, created_at, updated_at, shared, design FROM library.entries
WHERE owner_subject = @owner_subject AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
ORDER BY kind, lower(name), id;

-- name: LibraryRevisions :many
SELECT no, name, fields, author_subject, created_at, design, origin, client FROM library.entry_revisions WHERE entry_id = @entry_id ORDER BY no DESC;

-- name: LibraryRevision :one
SELECT name, fields, design FROM library.entry_revisions WHERE entry_id = @entry_id AND no = @no;

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
SELECT e.id, e.owner_subject, e.kind, e.name, e.fields, e.revision, e.created_at, e.updated_at, e.shared, e.design,
    l.pinned_revision, coalesce(l.override, '{}'::jsonb)::jsonb AS override, coalesce(l.direct, false)::boolean AS direct,
    r.name AS pinned_name, r.fields AS pinned_fields, r.design AS pinned_design,
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

-- name: CampaignHome :one
SELECT collection_id FROM library.campaign_homes WHERE campaign_id = @campaign_id;

-- name: InsertCampaignHome :exec
INSERT INTO library.campaign_homes (campaign_id, collection_id) VALUES (@campaign_id, @collection_id);

-- name: InsertProposal :exec
INSERT INTO library.proposals (id, campaign_id, author_subject, author_name, kind, name, fields, note, base_entry_id, status, created_at, updated_at)
VALUES (@id, @campaign_id, @author_subject, @author_name, @kind, @name, @fields, @note, sqlc.narg(base_entry_id), 'pending', @now, @now);

-- name: UpdateProposal :exec
UPDATE library.proposals SET name = @name, fields = @fields, note = @note, status = @status, message = @message,
    entry_id = sqlc.narg(entry_id), updated_at = @now
WHERE id = @id;

-- name: Proposal :one
SELECT id, campaign_id, author_subject, author_name, kind, name, fields, note, base_entry_id, status, message, entry_id, created_at, updated_at
FROM library.proposals WHERE id = @id;

-- name: CampaignProposals :many
SELECT id, campaign_id, author_subject, author_name, kind, name, fields, note, base_entry_id, status, message, entry_id, created_at, updated_at
FROM library.proposals
WHERE campaign_id = @campaign_id AND (sqlc.narg(author_subject)::text IS NULL OR author_subject = sqlc.narg(author_subject)::text)
ORDER BY created_at DESC, id;

-- name: InsertProposalReview :exec
INSERT INTO library.proposal_reviews (proposal_id, no, action, message, by_name, created_at)
SELECT @proposal_id, coalesce(max(no), 0) + 1, @action, @message, @by_name, @now FROM library.proposal_reviews WHERE proposal_id = @proposal_id;

-- name: ProposalReviews :many
SELECT no, action, message, by_name, created_at FROM library.proposal_reviews WHERE proposal_id = @proposal_id ORDER BY no;

-- name: InsertSharedSubmission :exec
INSERT INTO library.shared_submissions (id, entry_id, revision, kind, name, fields, note, submitter_subject, status, created_at, design)
VALUES (@id, @entry_id, @revision, @kind, @name, @fields, @note, @submitter_subject, 'pending', @now, sqlc.narg(design));

-- name: DecideSharedSubmission :exec
UPDATE library.shared_submissions SET status = @status, ip_clear = @ip_clear, ip_note = @ip_note, message = @message,
    reviewer_subject = @reviewer_subject, shared_entry_id = sqlc.narg(shared_entry_id), decided_at = @now
WHERE id = @id;

-- name: SharedSubmission :one
SELECT id, entry_id, revision, kind, name, fields, note, submitter_subject, status, ip_clear, ip_note, message, reviewer_subject,
    shared_entry_id, created_at, decided_at, design
FROM library.shared_submissions WHERE id = @id;

-- name: SharedSubmissions :many
SELECT id, entry_id, revision, kind, name, fields, note, submitter_subject, status, ip_clear, ip_note, message, reviewer_subject,
    shared_entry_id, created_at, decided_at, design
FROM library.shared_submissions
WHERE sqlc.narg(submitter_subject)::text IS NULL OR submitter_subject = sqlc.narg(submitter_subject)::text
ORDER BY status = 'pending' DESC, created_at DESC, id;

-- name: PendingSharedSubmission :one
SELECT EXISTS (SELECT 1 FROM library.shared_submissions WHERE entry_id = @entry_id AND status = 'pending');
