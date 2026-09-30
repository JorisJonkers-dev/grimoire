-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What a token needs for ambushes: its Stealth and Perception bonuses, initiative bonus and walking speed.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS stealth integer NOT NULL DEFAULT 0;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS perception integer NOT NULL DEFAULT 0;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS initiative integer NOT NULL DEFAULT 0;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS speed_ft integer NOT NULL DEFAULT 30;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_ambush_check CHECK (stealth BETWEEN -10 AND 30 AND perception BETWEEN -10 AND 30
    AND initiative BETWEEN -10 AND 20 AND speed_ft BETWEEN 0 AND 120) NOT VALID;

ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS surprised boolean NOT NULL DEFAULT false;

-- An Encounter Zone: hidden creatures around a hex that spring when the party comes within range.
CREATE TABLE IF NOT EXISTS play.encounter_zones (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    q integer NOT NULL,
    r integer NOT NULL,
    radius_hexes integer NOT NULL CHECK (radius_hexes BETWEEN 1 AND 20),
    dm_only boolean NOT NULL,
    held boolean NOT NULL,
    status text NOT NULL CHECK (status IN ('armed', 'spotting', 'sprung')),
    dc integer NOT NULL CHECK (dc BETWEEN 0 AND 50)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_zones_session_idx ON play.encounter_zones (session_id);

-- The hidden creatures a zone held when it sprang.
CREATE TABLE IF NOT EXISTS play.zone_creatures (
    zone_id uuid NOT NULL REFERENCES play.encounter_zones (id) ON DELETE CASCADE,
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    PRIMARY KEY (zone_id, token_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS zone_creatures_token_idx ON play.zone_creatures (token_id);

-- Who in the party noticed a sprung zone: by passive Perception, or by a Perception roll still to come.
CREATE TABLE IF NOT EXISTS play.zone_checks (
    zone_id uuid NOT NULL REFERENCES play.encounter_zones (id) ON DELETE CASCADE,
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    roll_id uuid REFERENCES play.roll_requests (id) ON DELETE SET NULL,
    noticed boolean,
    PRIMARY KEY (zone_id, token_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS zone_checks_token_idx ON play.zone_checks (token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS zone_checks_roll_idx ON play.zone_checks (roll_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set', 'table_set', 'world_set', 'node_added',
    'node_removed', 'route_added', 'route_removed', 'party_placed', 'travel_leg', 'zone_added',
    'zone_removed', 'zone_held', 'zone_sprung', 'perception_rolled')) NOT VALID;
