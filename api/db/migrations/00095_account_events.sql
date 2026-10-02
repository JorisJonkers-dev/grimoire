-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An Account's history: what happened to it and who did it, for the Admin page and the holder.
CREATE TABLE IF NOT EXISTS identity.account_events (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    actor text NOT NULL CHECK (char_length(actor) BETWEEN 1 AND 200),
    action text NOT NULL CHECK (char_length(action) BETWEEN 1 AND 40),
    detail text NOT NULL DEFAULT '' CHECK (char_length(detail) <= 200),
    at timestamptz NOT NULL
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS account_events_account_idx ON identity.account_events (account_id, at DESC);
