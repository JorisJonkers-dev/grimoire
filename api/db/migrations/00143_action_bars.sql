-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- How a player laid out a Character's action bars. It lives on the Character, not the Campaign
-- Character, so it follows the Character into every Campaign; NULL until the player first arranges them.
ALTER TABLE campaign.account_characters ADD COLUMN IF NOT EXISTS action_bars jsonb;

