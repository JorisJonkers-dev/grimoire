-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- How each Revision of a Library entry was made: by hand in the app or through an MCP client, and
-- which one. A Revision made before this was made by hand.
ALTER TABLE library.entry_revisions ADD COLUMN IF NOT EXISTS origin text NOT NULL DEFAULT 'ui';
ALTER TABLE library.entry_revisions ADD COLUMN IF NOT EXISTS client text;
ALTER TABLE library.entry_revisions ADD CONSTRAINT entry_revisions_origin_check
    CHECK (origin IN ('ui', 'mcp', 'generator', 'system') AND (client IS NULL OR char_length(client) BETWEEN 1 AND 80)) NOT VALID;
