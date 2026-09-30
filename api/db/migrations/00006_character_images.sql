-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS portrait_key text;
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS portrait_type text;
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS token_key text;
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS token_type text;
