-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A device that asked to be told about turns and Reaction Prompts, by the account it signed in with.
CREATE TABLE IF NOT EXISTS campaign.push_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 200),
    endpoint text NOT NULL UNIQUE CHECK (char_length(endpoint) BETWEEN 1 AND 2000),
    p256dh text NOT NULL CHECK (char_length(p256dh) BETWEEN 1 AND 200),
    auth text NOT NULL CHECK (char_length(auth) BETWEEN 1 AND 100),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS push_subscriptions_subject_idx ON campaign.push_subscriptions (subject);
