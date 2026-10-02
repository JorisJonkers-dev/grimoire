-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Notifications waiting to go out by email in the next Digest; the latest with a dedupe key wins.
CREATE TABLE IF NOT EXISTS social.email_queue (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 40),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    body text NOT NULL DEFAULT '' CHECK (char_length(body) <= 200),
    action_label text NOT NULL CHECK (char_length(action_label) BETWEEN 1 AND 20),
    action_path text NOT NULL CHECK (action_path LIKE '/%' AND char_length(action_path) <= 300),
    dedupe_key text NOT NULL DEFAULT '' CHECK (char_length(dedupe_key) <= 120),
    created_at timestamptz NOT NULL
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS email_queue_account_idx ON social.email_queue (account_id, created_at);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS email_queue_dedupe_idx ON social.email_queue (account_id, dedupe_key) WHERE dedupe_key <> '';

-- When each Account last got a Digest, so it gets one at most hourly.
CREATE TABLE IF NOT EXISTS social.digests (
    account_id uuid PRIMARY KEY REFERENCES identity.accounts (id) ON DELETE CASCADE,
    sent_at timestamptz NOT NULL
);
