-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Track: a Campaign-specific score such as sanity or renown, kept for each Character or for the party.
CREATE TABLE IF NOT EXISTS campaign.tracks (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    scope text NOT NULL CHECK (scope IN ('character', 'party')),
    lowest integer NOT NULL CHECK (lowest BETWEEN -1000 AND 1000),
    highest integer NOT NULL CHECK (highest BETWEEN -1000 AND 1000),
    start integer NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT tracks_bounds_check CHECK (lowest < highest AND start BETWEEN lowest AND highest)
);
CREATE INDEX IF NOT EXISTS tracks_campaign_idx ON campaign.tracks (campaign_id, created_at);

-- A score at which something happens, on the way up or on the way down: an Effect, a roll on a Roll
-- Table, or only its label.
CREATE TABLE IF NOT EXISTS campaign.track_thresholds (
    track_id uuid NOT NULL REFERENCES campaign.tracks (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 19),
    at integer NOT NULL,
    rising boolean NOT NULL,
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    effect text CHECK (char_length(effect) BETWEEN 1 AND 80),
    roll_table uuid,
    PRIMARY KEY (track_id, position),
    CONSTRAINT track_thresholds_one_outcome_check CHECK (effect IS NULL OR roll_table IS NULL)
);

-- Where a Track stands: for one Character, or for the party when no Character is named. A Track with
-- no row stands at its start.
CREATE TABLE IF NOT EXISTS campaign.track_values (
    track_id uuid NOT NULL REFERENCES campaign.tracks (id) ON DELETE CASCADE,
    character_id uuid REFERENCES campaign.characters (id) ON DELETE CASCADE,
    value integer NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS track_values_character_idx ON campaign.track_values (track_id, character_id) WHERE character_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS track_values_party_idx ON campaign.track_values (track_id) WHERE character_id IS NULL;
CREATE INDEX IF NOT EXISTS track_values_of_character_idx ON campaign.track_values (character_id) WHERE character_id IS NOT NULL;
