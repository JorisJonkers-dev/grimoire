-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE SCHEMA IF NOT EXISTS library;

-- A Library entry belongs to the account that made it and is linked into Campaigns, never copied.
-- Its fields are named values; revision is the number of its latest Revision.
CREATE TABLE IF NOT EXISTS library.entries (
    id uuid PRIMARY KEY,
    owner_subject text NOT NULL CHECK (length(owner_subject) BETWEEN 1 AND 255),
    kind text NOT NULL CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table')),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    fields jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(fields) = 'object'),
    revision integer NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS entries_owner_idx ON library.entries (owner_subject, kind, name);

-- Every saved version of an entry's base, numbered from 1.
CREATE TABLE IF NOT EXISTS library.entry_revisions (
    entry_id uuid NOT NULL REFERENCES library.entries (id) ON DELETE CASCADE,
    no integer NOT NULL CHECK (no >= 1),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    fields jsonb NOT NULL CHECK (jsonb_typeof(fields) = 'object'),
    author_subject text NOT NULL CHECK (length(author_subject) BETWEEN 1 AND 255),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (entry_id, no)
);

-- An entry linked into a Campaign, with the Campaign Override layered on its base and the Revision the
-- Campaign is pinned to, if any.
CREATE TABLE IF NOT EXISTS library.campaign_links (
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES library.entries (id) ON DELETE CASCADE,
    pinned_revision integer CHECK (pinned_revision >= 1),
    override jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(override) = 'object'),
    linked_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (campaign_id, entry_id),
    FOREIGN KEY (entry_id, pinned_revision) REFERENCES library.entry_revisions (entry_id, no)
);
CREATE INDEX IF NOT EXISTS campaign_links_entry_idx ON library.campaign_links (entry_id);
