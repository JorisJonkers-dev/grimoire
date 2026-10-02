-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A shared entry is a read-only copy in the Shared Library that every DM can link.
ALTER TABLE library.entries ADD COLUMN IF NOT EXISTS shared boolean NOT NULL DEFAULT false;

-- A DM's request to share one Revision of an entry, and the Admin's review of it: approval needs the
-- IP check that the entry carries no non-SRD text.
CREATE TABLE IF NOT EXISTS library.shared_submissions (
    id uuid PRIMARY KEY,
    entry_id uuid NOT NULL REFERENCES library.entries (id) ON DELETE CASCADE,
    revision integer NOT NULL CHECK (revision >= 1),
    kind text NOT NULL CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table')),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    fields jsonb NOT NULL CHECK (jsonb_typeof(fields) = 'object'),
    note text NOT NULL DEFAULT '' CHECK (length(note) <= 2000),
    submitter_subject text NOT NULL CHECK (length(submitter_subject) BETWEEN 1 AND 255),
    status text NOT NULL CHECK (status IN ('pending', 'approved', 'declined')),
    ip_clear boolean,
    ip_note text NOT NULL DEFAULT '' CHECK (length(ip_note) <= 2000),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
    reviewer_subject text,
    shared_entry_id uuid REFERENCES library.entries (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    decided_at timestamptz,
    CHECK (status <> 'approved' OR ip_clear)
);
CREATE INDEX IF NOT EXISTS shared_submissions_entry_idx ON library.shared_submissions (entry_id);
CREATE INDEX IF NOT EXISTS shared_submissions_shared_idx ON library.shared_submissions (shared_entry_id);
CREATE INDEX IF NOT EXISTS shared_submissions_status_idx ON library.shared_submissions (status, created_at);
