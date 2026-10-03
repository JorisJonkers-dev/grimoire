-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A homebrew condition is a Library entry too, built in the condition builder.
ALTER TABLE library.entries DROP CONSTRAINT IF EXISTS entries_kind_check;
ALTER TABLE library.entries ADD CONSTRAINT entries_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class', 'species', 'background', 'feat', 'condition')) NOT VALID;
ALTER TABLE library.proposals DROP CONSTRAINT IF EXISTS proposals_kind_check;
ALTER TABLE library.proposals ADD CONSTRAINT proposals_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class', 'species', 'background', 'feat', 'condition')) NOT VALID;
ALTER TABLE library.shared_submissions DROP CONSTRAINT IF EXISTS shared_submissions_kind_check;
ALTER TABLE library.shared_submissions ADD CONSTRAINT shared_submissions_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class', 'species', 'background', 'feat', 'condition')) NOT VALID;

-- The exhaustion variant a Campaign plays with.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS exhaustion_variant text NOT NULL DEFAULT 'srd-2024';
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_exhaustion_variant_check
    CHECK (exhaustion_variant IN ('srd-2024', 'gentle', 'grim', 'off')) NOT VALID;
