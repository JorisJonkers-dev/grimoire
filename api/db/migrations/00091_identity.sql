-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Grimoire owns its Accounts (ADR-0008). An Account's subject is the identity the rest of the app keys
-- on: "account:<id>" for an internal Account.
CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE IF NOT EXISTS identity.accounts (
    id uuid PRIMARY KEY,
    subject text NOT NULL UNIQUE CHECK (char_length(subject) BETWEEN 1 AND 200),
    username text NOT NULL CHECK (username ~ '^[a-z0-9][a-z0-9_.-]{2,31}$'),
    nickname text NOT NULL CHECK (char_length(nickname) BETWEEN 1 AND 40),
    email text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254 AND email LIKE '%_@_%'),
    password_hash text CHECK (char_length(password_hash) <= 300),
    admin boolean NOT NULL DEFAULT false,
    disabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL
);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS accounts_username_idx ON identity.accounts (username);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS accounts_email_idx ON identity.accounts (lower(email));

-- An Account Invite: an Admin's one-time link to set up an Account, closed once used or expired.
CREATE TABLE IF NOT EXISTS identity.invites (
    id uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_by text NOT NULL CHECK (char_length(created_by) BETWEEN 1 AND 200),
    admin boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    account_id uuid REFERENCES identity.accounts (id) ON DELETE SET NULL,
    CHECK (expires_at > created_at)
);

-- A signed-in device: an opaque token in a cookie, stored only as its hash.
CREATE TABLE IF NOT EXISTS identity.account_sessions (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    user_agent text NOT NULL CHECK (char_length(user_agent) <= 300),
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS account_sessions_account_idx ON identity.account_sessions (account_id);

-- An emailed sign-in link for a forgotten password, good once and briefly.
CREATE TABLE IF NOT EXISTS identity.sign_in_links (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS sign_in_links_account_idx ON identity.sign_in_links (account_id);
