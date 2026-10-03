-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A homebrew class is a Library entry too, built in the class builder.
ALTER TABLE library.entries DROP CONSTRAINT IF EXISTS entries_kind_check;
ALTER TABLE library.entries ADD CONSTRAINT entries_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class')) NOT VALID;
ALTER TABLE library.proposals DROP CONSTRAINT IF EXISTS proposals_kind_check;
ALTER TABLE library.proposals ADD CONSTRAINT proposals_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class')) NOT VALID;
ALTER TABLE library.shared_submissions DROP CONSTRAINT IF EXISTS shared_submissions_kind_check;
ALTER TABLE library.shared_submissions ADD CONSTRAINT shared_submissions_kind_check
    CHECK (kind IN ('creature', 'npc', 'location', 'shop', 'item', 'spell', 'table', 'subclass', 'class')) NOT VALID;
