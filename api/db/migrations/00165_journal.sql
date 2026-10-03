-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Quest: a goal the party pursues, with steps and a status. A hidden one is the DM's until it is given.
CREATE TABLE IF NOT EXISTS campaign.quests (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    summary text NOT NULL CHECK (char_length(summary) <= 4000),
    status text NOT NULL CHECK (status IN ('hidden', 'active', 'completed', 'failed')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS quests_campaign_idx ON campaign.quests (campaign_id);

CREATE TABLE IF NOT EXISTS campaign.quest_steps (
    quest_id uuid NOT NULL REFERENCES campaign.quests (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 49),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 400),
    done boolean NOT NULL,
    PRIMARY KEY (quest_id, position)
);

-- Lore: world knowledge the party unlocks, for example by reading a book or letter it carries. Until
-- it is unlocked it is the DM's alone.
CREATE TABLE IF NOT EXISTS campaign.lore (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    body text NOT NULL CHECK (char_length(body) <= 8000),
    item_slug text NOT NULL CHECK (char_length(item_slug) <= 80),
    unlocked_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS lore_campaign_idx ON campaign.lore (campaign_id);
