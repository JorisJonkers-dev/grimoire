-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS play.combats (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('rolling', 'active', 'ended')),
    round integer NOT NULL CHECK (round >= 0),
    turn_count integer,
    started_at timestamptz NOT NULL,
    ended_at timestamptz
);

-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS combats_one_running_idx ON play.combats (session_id) WHERE status <> 'ended';

CREATE TABLE IF NOT EXISTS play.combatants (
    id uuid PRIMARY KEY,
    combat_id uuid NOT NULL REFERENCES play.combats (id) ON DELETE CASCADE,
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id),
    initiative_bonus integer NOT NULL CHECK (initiative_bonus BETWEEN -10 AND 20),
    speed_ft integer NOT NULL CHECK (speed_ft BETWEEN 0 AND 120),
    initiative integer,
    done boolean NOT NULL DEFAULT false,
    has_action boolean NOT NULL DEFAULT false,
    has_bonus_action boolean NOT NULL DEFAULT false,
    has_reaction boolean NOT NULL DEFAULT false,
    movement_ft integer NOT NULL DEFAULT 0 CHECK (movement_ft BETWEEN 0 AND 120),
    UNIQUE (combat_id, token_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS combatants_token_idx ON play.combatants (token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS combatants_roll_idx ON play.combatants (roll_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended')) NOT VALID;
