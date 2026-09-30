-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS play.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    number integer NOT NULL CHECK (number >= 1),
    status text NOT NULL CHECK (status IN ('live', 'ended')),
    seq bigint NOT NULL DEFAULT 0 CHECK (seq >= 0),
    grid_radius integer NOT NULL DEFAULT 10 CHECK (grid_radius BETWEEN 1 AND 60),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    UNIQUE (campaign_id, number),
    CHECK ((status = 'ended') = (ended_at IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS play.tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 40),
    kind text NOT NULL CHECK (kind IN ('party', 'enemy', 'npc', 'object')),
    q integer NOT NULL,
    r integer NOT NULL,
    hidden boolean NOT NULL DEFAULT false
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS tokens_session_idx ON play.tokens (session_id);

ALTER TABLE play.actions ADD COLUMN IF NOT EXISTS session_id uuid;
ALTER TABLE play.actions ADD CONSTRAINT actions_session_fk FOREIGN KEY (session_id) REFERENCES play.sessions (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed')) NOT VALID;
ALTER TABLE play.actions ADD CONSTRAINT actions_seed_check CHECK ((kind = 'die_rolled') = (seed IS NOT NULL)) NOT VALID;

CREATE TABLE IF NOT EXISTS play.action_token_events (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    token_id uuid NOT NULL,
    label text NOT NULL,
    q integer NOT NULL,
    r integer NOT NULL,
    hidden boolean NOT NULL
);
