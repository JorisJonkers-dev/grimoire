-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An authenticator app an Account signs in with as a second step: its shared secret, when it was
-- confirmed with a first code, and the last time step a code was used for, so no code works twice.
CREATE TABLE IF NOT EXISTS identity.totp_factors (
    account_id uuid PRIMARY KEY REFERENCES identity.accounts (id) ON DELETE CASCADE,
    secret text NOT NULL CHECK (char_length(secret) = 32),
    created_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    last_step bigint NOT NULL DEFAULT 0
);

-- Recovery codes for a lost authenticator, each good once, stored only as hashes.
CREATE TABLE IF NOT EXISTS identity.recovery_codes (
    code_hash bytea PRIMARY KEY CHECK (octet_length(code_hash) = 32),
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    used_at timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS recovery_codes_account_idx ON identity.recovery_codes (account_id);

-- A password or emailed-link sign-in waiting for its second step, with the wrong codes tried so far.
CREATE TABLE IF NOT EXISTS identity.two_step_challenges (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0)
);

-- A strong session passed a second step or came from the external login; only a strong session holds
-- Admin powers.
ALTER TABLE identity.account_sessions ADD COLUMN IF NOT EXISTS strong boolean NOT NULL DEFAULT false;
