-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Temporary hit points a Campaign Character holds outside play; they soak damage first.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS temp_hp integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_temp_hp_check CHECK (temp_hp BETWEEN 0 AND 1000) NOT VALID;
