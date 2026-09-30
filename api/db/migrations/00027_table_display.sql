-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What the Table Display shows, steered by the DM: camera, scene and blackout.
CREATE TABLE IF NOT EXISTS play.table_displays (
    session_id uuid PRIMARY KEY REFERENCES play.sessions (id) ON DELETE CASCADE,
    camera text NOT NULL CHECK (camera IN ('follow_turn', 'show_party', 'free')),
    q integer NOT NULL,
    r integer NOT NULL,
    zoom_pct integer NOT NULL CHECK (zoom_pct BETWEEN 50 AND 300),
    scene text NOT NULL CHECK (scene IN ('local', 'world', 'handout', 'title')),
    title text NOT NULL CHECK (char_length(title) <= 80),
    body text NOT NULL CHECK (char_length(body) <= 1000),
    map_id uuid REFERENCES campaign.maps (id) ON DELETE SET NULL,
    blackout boolean NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS table_displays_map_idx ON play.table_displays (map_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set', 'table_set')) NOT VALID;
