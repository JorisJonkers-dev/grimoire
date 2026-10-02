-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Conversation between Friends outside a Session, one-to-one or a small group.
CREATE TABLE IF NOT EXISTS social.conversations (
    id uuid PRIMARY KEY,
    title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 80),
    created_by uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- Who is in a Conversation, and the last time each of them read it.
CREATE TABLE IF NOT EXISTS social.conversation_members (
    conversation_id uuid NOT NULL REFERENCES social.conversations (id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    joined_at timestamptz NOT NULL,
    last_read_at timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, account_id)
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS conversation_members_account_idx ON social.conversation_members (account_id);

CREATE TABLE IF NOT EXISTS social.messages (
    id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES social.conversations (id) ON DELETE CASCADE,
    author uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 4000),
    created_at timestamptz NOT NULL
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS messages_conversation_idx ON social.messages (conversation_id, created_at DESC);

-- Game content a Message points at: a Campaign Character or a Location, in its Campaign.
CREATE TABLE IF NOT EXISTS social.message_mentions (
    message_id uuid NOT NULL REFERENCES social.messages (id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal BETWEEN 0 AND 9),
    kind text NOT NULL CHECK (kind IN ('character', 'location')),
    campaign_id uuid NOT NULL,
    target_id uuid NOT NULL,
    PRIMARY KEY (message_id, ordinal)
);
