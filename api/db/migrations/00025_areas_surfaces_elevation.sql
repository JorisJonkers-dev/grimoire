-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Terrain height belongs to the Map, so it carries over from Session to Session.
CREATE TABLE IF NOT EXISTS campaign.map_elevations (
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    elevation_ft integer NOT NULL CHECK (elevation_ft BETWEEN -100 AND 100 AND elevation_ft <> 0),
    PRIMARY KEY (map_id, q, r)
);

ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS high_ground boolean NOT NULL DEFAULT false;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS spell_dc integer;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_spell_dc_check CHECK (spell_dc BETWEEN 1 AND 40) NOT VALID;

CREATE TABLE IF NOT EXISTS play.surfaces (
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    kind text NOT NULL CHECK (kind IN ('fire', 'grease', 'water', 'ice', 'web', 'electrified')),
    rounds_left integer CHECK (rounds_left > 0),
    PRIMARY KEY (session_id, q, r)
);

-- The area spell waiting on its damage and saving throws; one at a time per Session.
CREATE TABLE IF NOT EXISTS play.area_casts (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL UNIQUE REFERENCES play.sessions (id) ON DELETE CASCADE,
    caster_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    spell text NOT NULL,
    dc integer NOT NULL CHECK (dc BETWEEN 1 AND 40),
    damage_roll_id uuid REFERENCES play.roll_requests (id) ON DELETE CASCADE
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS area_casts_caster_idx ON play.area_casts (caster_token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS area_casts_roll_idx ON play.area_casts (damage_roll_id);

CREATE TABLE IF NOT EXISTS play.area_hexes (
    cast_id uuid NOT NULL REFERENCES play.area_casts (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    PRIMARY KEY (cast_id, q, r)
);

CREATE TABLE IF NOT EXISTS play.area_targets (
    cast_id uuid NOT NULL REFERENCES play.area_casts (id) ON DELETE CASCADE,
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    save_roll_id uuid REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    PRIMARY KEY (cast_id, token_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS area_targets_token_idx ON play.area_targets (token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS area_targets_roll_idx ON play.area_targets (save_roll_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set')) NOT VALID;
