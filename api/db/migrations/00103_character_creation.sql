-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- How a Campaign lets players make Characters: the ability score methods it allows and the level new
-- Characters start at.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS creation_methods text [] NOT NULL DEFAULT ARRAY['standard-array', 'point-buy', 'rolled'];
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS starting_level integer NOT NULL DEFAULT 1;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_creation_methods_check
CHECK (cardinality(creation_methods) BETWEEN 1 AND 3 AND creation_methods <@ ARRAY['standard-array', 'point-buy', 'rolled']) NOT VALID;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_starting_level_check CHECK (starting_level BETWEEN 1 AND 20) NOT VALID;

-- What a Character looks like, written in the creation wizard.
ALTER TABLE campaign.account_characters ADD COLUMN IF NOT EXISTS appearance text NOT NULL DEFAULT '';
ALTER TABLE campaign.account_characters ADD CONSTRAINT account_characters_appearance_check CHECK (char_length(appearance) <= 2000) NOT VALID;

-- A Character being made in the wizard: the step reached, the choices so far, and the six scores the
-- server rolled for it, once.
CREATE TABLE IF NOT EXISTS campaign.character_drafts (
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    owner_subject text NOT NULL CHECK (char_length(owner_subject) BETWEEN 1 AND 200),
    step integer NOT NULL CHECK (step BETWEEN 0 AND 8),
    build jsonb NOT NULL,
    rolled integer [] CHECK (rolled IS NULL OR cardinality(rolled) = 6),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (campaign_id, owner_subject)
);
