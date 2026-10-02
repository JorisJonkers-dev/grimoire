-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Notification in an Account's bell, with the one action it offers. An unread Notification with a
-- dedupe key is replaced by the next of its kind, so one Conversation shows once however busy it gets.
CREATE TABLE IF NOT EXISTS social.notifications (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('proposal', 'join_request', 'level_up', 'friend_request', 'conversation', 'session_reminder', 'release_note', 'security')),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    body text NOT NULL DEFAULT '' CHECK (char_length(body) <= 200),
    action_label text NOT NULL CHECK (char_length(action_label) BETWEEN 1 AND 20),
    action_path text NOT NULL CHECK (action_path LIKE '/%' AND char_length(action_path) <= 300),
    dedupe_key text NOT NULL DEFAULT '' CHECK (char_length(dedupe_key) <= 120),
    created_at timestamptz NOT NULL,
    read_at timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS notifications_account_idx ON social.notifications (account_id, created_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS notifications_unread_dedupe_idx ON social.notifications (account_id, dedupe_key)
WHERE read_at IS NULL AND dedupe_key <> '';

-- Which Notification kinds reach an Account on which channel; a missing row takes the default.
CREATE TABLE IF NOT EXISTS social.notification_preferences (
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('proposal', 'join_request', 'level_up', 'friend_request', 'conversation', 'session_reminder', 'release_note', 'security')),
    channel text NOT NULL CHECK (channel IN ('in_app', 'push', 'email')),
    enabled boolean NOT NULL,
    PRIMARY KEY (account_id, kind, channel)
);
