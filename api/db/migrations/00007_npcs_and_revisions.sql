-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS campaign.npcs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 8000),
    dm_notes text NOT NULL DEFAULT '' CHECK (char_length(dm_notes) <= 8000),
    disposition text NOT NULL DEFAULT 'neutral' CHECK (disposition IN ('friendly', 'neutral', 'hostile')),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS npcs_campaign_idx ON campaign.npcs (campaign_id, name);

-- Revisions outlive the rows they describe, so a deleted entity can be restored: entity_id has no FK.
CREATE TABLE IF NOT EXISTS campaign.revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    entity_type text NOT NULL CHECK (entity_type IN ('npc')),
    entity_id uuid NOT NULL,
    revision_no integer NOT NULL CHECK (revision_no >= 1),
    action text NOT NULL CHECK (action IN ('create', 'update', 'delete', 'restore')),
    caller_subject text NOT NULL,
    caller_name text NOT NULL,
    origin text NOT NULL CHECK (origin IN ('ui', 'mcp', 'generator', 'system')),
    client text NOT NULL DEFAULT '',
    restored_from integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (entity_type, entity_id, revision_no),
    CHECK ((action = 'restore') = (restored_from IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS revisions_campaign_idx ON campaign.revisions (campaign_id, entity_type, created_at DESC);

CREATE TABLE IF NOT EXISTS campaign.npc_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    name text NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    dm_notes text NOT NULL,
    disposition text NOT NULL
);
