-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An OIDC login linked to an Account (ADR-0008): at most one per Account and one Account per login. The
-- claims are what the provider last said, shown read-only.
CREATE TABLE IF NOT EXISTS identity.oidc_links (
    account_id uuid PRIMARY KEY REFERENCES identity.accounts (id) ON DELETE CASCADE,
    issuer text NOT NULL CHECK (char_length(issuer) BETWEEN 1 AND 300),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    email text NOT NULL CHECK (char_length(email) <= 254),
    username text NOT NULL CHECK (char_length(username) <= 200),
    name text NOT NULL CHECK (char_length(name) <= 200),
    linked_at timestamptz NOT NULL,
    UNIQUE (issuer, subject)
);

-- A sign-in sent to the provider: the state it comes back with, stored as its hash, the nonce and PKCE
-- verifier it was sent with, and the Account it links to when started from the Account page.
CREATE TABLE IF NOT EXISTS identity.oidc_requests (
    state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    nonce text NOT NULL CHECK (char_length(nonce) BETWEEN 20 AND 64),
    verifier text NOT NULL CHECK (char_length(verifier) BETWEEN 43 AND 128),
    account_id uuid REFERENCES identity.accounts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);

-- A login the provider vouched for that no Account has yet, waiting for its holder to create an Account
-- or link one.
CREATE TABLE IF NOT EXISTS identity.oidc_pending (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    issuer text NOT NULL CHECK (char_length(issuer) BETWEEN 1 AND 300),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    email text NOT NULL CHECK (char_length(email) <= 254),
    username text NOT NULL CHECK (char_length(username) <= 200),
    name text NOT NULL CHECK (char_length(name) <= 200),
    admin boolean NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);

