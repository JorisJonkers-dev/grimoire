-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An Access Token (ADR-0009): a bearer credential for MCP clients and scripts that acts as its Account,
-- narrowed to scopes and an expiry. Only the token's hash is kept; the token is shown once.
CREATE TABLE IF NOT EXISTS identity.access_tokens (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    scopes text [] NOT NULL CHECK (cardinality(scopes) BETWEEN 1 AND 3 AND scopes <@ ARRAY['read', 'build', 'play']),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS access_tokens_account_idx ON identity.access_tokens (account_id);
