-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Each Campaign's own Collection, for entries made or approved just for it.
CREATE TABLE IF NOT EXISTS library.campaign_homes (
    campaign_id uuid PRIMARY KEY REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    collection_id uuid NOT NULL UNIQUE REFERENCES library.collections (id) ON DELETE CASCADE
);

-- A Player's request that the DM accept a new entry, or a change to one the Campaign sees.
CREATE TABLE IF NOT EXISTS library.proposals (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    author_subject text NOT NULL CHECK (length(author_subject) BETWEEN 1 AND 255),
    author_name text NOT NULL CHECK (length(author_name) BETWEEN 1 AND 80),
    kind text NOT NULL CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table')),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    fields jsonb NOT NULL CHECK (jsonb_typeof(fields) = 'object'),
    note text NOT NULL DEFAULT '' CHECK (length(note) <= 2000),
    base_entry_id uuid REFERENCES library.entries (id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('pending', 'changes_requested', 'approved', 'declined')),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
    entry_id uuid REFERENCES library.entries (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS proposals_campaign_idx ON library.proposals (campaign_id, created_at);
CREATE INDEX IF NOT EXISTS proposals_base_idx ON library.proposals (base_entry_id);
CREATE INDEX IF NOT EXISTS proposals_entry_idx ON library.proposals (entry_id);

-- What happened to a Proposal, in order: sent, sent again, changes asked for, approved or declined.
CREATE TABLE IF NOT EXISTS library.proposal_reviews (
    proposal_id uuid NOT NULL REFERENCES library.proposals (id) ON DELETE CASCADE,
    no integer NOT NULL CHECK (no >= 1),
    action text NOT NULL CHECK (action IN ('submitted', 'resubmitted', 'changes_requested', 'approved', 'declined')),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 2000),
    by_name text NOT NULL CHECK (length(by_name) BETWEEN 1 AND 80),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (proposal_id, no)
);
