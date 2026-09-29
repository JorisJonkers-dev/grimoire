-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS campaign.campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    ruleset_pref text NOT NULL DEFAULT 'srd-2024' CHECK (ruleset_pref IN ('srd-2024', 'srd-2014')),
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS campaign.members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    auth_subject text NOT NULL CHECK (char_length(auth_subject) BETWEEN 1 AND 128),
    display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 60),
    role text NOT NULL CHECK (role IN ('dm', 'player')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, auth_subject)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS members_auth_subject_idx ON campaign.members (auth_subject, campaign_id);

CREATE TABLE IF NOT EXISTS campaign.invites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_by uuid NOT NULL REFERENCES campaign.members (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS invites_campaign_idx ON campaign.invites (campaign_id, created_at DESC);
