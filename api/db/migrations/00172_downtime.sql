-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Downtime days are a Resource each Character holds and spends between adventures; spent counts the
-- days it has lived through in the downtime the DM last gave the whole party.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS downtime_days integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS downtime_spent integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_downtime_check
    CHECK (downtime_days BETWEEN 0 AND 100000 AND downtime_spent BETWEEN 0 AND 100000) NOT VALID;
-- How many days of that downtime the Game Clock has already moved on.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS downtime_advanced integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_downtime_advanced_check CHECK (downtime_advanced BETWEEN 0 AND 100000) NOT VALID;

-- A Recipe makes Items from ingredients, with a tool, over days, at a cost.
CREATE TABLE IF NOT EXISTS campaign.recipes (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    item_slug text NOT NULL CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100),
    tool_slug text CHECK (char_length(tool_slug) BETWEEN 1 AND 80),
    days integer NOT NULL CHECK (days BETWEEN 1 AND 365),
    cost_cp integer NOT NULL CHECK (cost_cp BETWEEN 0 AND 100000000),
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS recipes_campaign_idx ON campaign.recipes (campaign_id, created_at);

CREATE TABLE IF NOT EXISTS campaign.recipe_ingredients (
    recipe_id uuid NOT NULL REFERENCES campaign.recipes (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 19),
    item_slug text NOT NULL CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    count integer NOT NULL CHECK (count BETWEEN 1 AND 100),
    PRIMARY KEY (recipe_id, position)
);

-- What Characters did with their downtime.
CREATE TABLE IF NOT EXISTS campaign.downtime_log (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    activity text NOT NULL CHECK (activity IN ('craft', 'work', 'train', 'research')),
    detail text NOT NULL CHECK (char_length(detail) <= 200),
    days integer NOT NULL CHECK (days BETWEEN 1 AND 100000),
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS downtime_log_campaign_idx ON campaign.downtime_log (campaign_id, created_at);
CREATE INDEX IF NOT EXISTS downtime_log_character_idx ON campaign.downtime_log (character_id);
