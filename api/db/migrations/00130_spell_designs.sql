-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A homebrew spell's design: its Targeting and typed parts, as the Effect builder saves them.
ALTER TABLE library.entries ADD COLUMN IF NOT EXISTS design jsonb;
ALTER TABLE library.entry_revisions ADD COLUMN IF NOT EXISTS design jsonb;
ALTER TABLE library.shared_submissions ADD COLUMN IF NOT EXISTS design jsonb;
