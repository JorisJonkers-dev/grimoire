-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Release Note: one per full release, drafted by an Admin, live from its publish time.
CREATE TABLE IF NOT EXISTS social.release_notes (
    id uuid PRIMARY KEY,
    version text NOT NULL UNIQUE CHECK (version ~ '^[0-9]+\.[0-9]+\.[0-9]+$'),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    body text NOT NULL CHECK (char_length(body) <= 8000),
    created_by text NOT NULL CHECK (char_length(created_by) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    publish_at timestamptz,
    announced_at timestamptz
);

-- Which Accounts have seen a Release Note on their Dashboard, so each sees it once.
CREATE TABLE IF NOT EXISTS social.release_note_views (
    note_id uuid NOT NULL REFERENCES social.release_notes (id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    seen_at timestamptz NOT NULL,
    PRIMARY KEY (note_id, account_id)
);
