-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Friends and Conversations live in their own schema; everything keys on Accounts.
CREATE SCHEMA IF NOT EXISTS social;

-- A Friend request waiting for an answer; declined requests are kept so they are not shown again.
CREATE TABLE IF NOT EXISTS social.friend_requests (
    id uuid PRIMARY KEY,
    from_account uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    to_account uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('pending', 'declined')),
    created_at timestamptz NOT NULL,
    decided_at timestamptz,
    CHECK (from_account <> to_account)
);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS friend_requests_pending_idx ON social.friend_requests (from_account, to_account) WHERE status = 'pending';
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS friend_requests_to_idx ON social.friend_requests (to_account, status);

-- A mutual friendship, stored once with the lower Account id first.
CREATE TABLE IF NOT EXISTS social.friendships (
    a uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    b uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    since timestamptz NOT NULL,
    PRIMARY KEY (a, b),
    CHECK (a < b)
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS friendships_b_idx ON social.friendships (b);

-- An Account that blocked another: the blocked Account's Friend requests no longer reach it.
CREATE TABLE IF NOT EXISTS social.blocks (
    blocker uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    blocked uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (blocker, blocked)
);
