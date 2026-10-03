-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A lingering injury a Character carries from one Session to the next, until its cure: the condition
-- it is, by its slug, and what cures it.
CREATE TABLE IF NOT EXISTS campaign.character_injuries (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    slug text NOT NULL CHECK (char_length(slug) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    cure text NOT NULL CHECK (char_length(cure) <= 80),
    level integer NOT NULL CHECK (level BETWEEN 0 AND 10),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (character_id, slug)
);
