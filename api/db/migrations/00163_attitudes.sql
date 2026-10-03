-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A creature's attitude towards one Character: an Influence check moves it.
CREATE TABLE IF NOT EXISTS play.token_attitudes (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    attitude text NOT NULL CHECK (attitude IN ('hostile', 'indifferent', 'friendly')),
    PRIMARY KEY (token_id, character_id)
);
CREATE INDEX IF NOT EXISTS token_attitudes_character_idx ON play.token_attitudes (character_id);

-- A Campaign shows the DC of a check on its Roll Card, or keeps it from the table.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS show_dcs boolean NOT NULL DEFAULT false;

-- An Influence check aimed at a creature waits on its roll like any other action that has a DC.
ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_action_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_action_check
    CHECK (action IN ('hide', 'grapple', 'shove_push', 'shove_prone', 'topple', 'concentration', 'stabilise', 'disarm', 'pick', 'force', 'influence')) NOT VALID;
