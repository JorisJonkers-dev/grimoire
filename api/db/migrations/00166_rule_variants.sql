-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What a Campaign has each Rule Variant at. A variant with no row plays as the rules do without it.
CREATE TABLE IF NOT EXISTS campaign.rule_variants (
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    variant text NOT NULL CHECK (char_length(variant) BETWEEN 1 AND 60),
    value text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 60),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (campaign_id, variant)
);

-- The Short Rests the party has taken since its last Long Rest, for the Short Rest cap.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS short_rests integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_short_rests_check CHECK (short_rests BETWEEN 0 AND 100000) NOT VALID;
