-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Map is a local tactical map or a world map of locations and routes.
ALTER TABLE campaign.maps ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'local';
ALTER TABLE campaign.maps ADD CONSTRAINT maps_kind_check CHECK (kind IN ('local', 'world')) NOT VALID;

CREATE TABLE IF NOT EXISTS campaign.map_nodes (
    id uuid PRIMARY KEY,
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    q integer NOT NULL,
    r integer NOT NULL,
    UNIQUE (map_id, q, r)
);

-- A route between two locations; travelling it is a Travel Leg.
CREATE TABLE IF NOT EXISTS campaign.map_edges (
    id uuid PRIMARY KEY,
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    from_node_id uuid NOT NULL REFERENCES campaign.map_nodes (id) ON DELETE CASCADE,
    to_node_id uuid NOT NULL REFERENCES campaign.map_nodes (id) ON DELETE CASCADE,
    distance_mi integer NOT NULL CHECK (distance_mi BETWEEN 1 AND 2000),
    CHECK (from_node_id <> to_node_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS map_edges_map_idx ON campaign.map_edges (map_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS map_edges_from_idx ON campaign.map_edges (from_node_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS map_edges_to_idx ON campaign.map_edges (to_node_id);

-- Where the party stands on each world map; it stays there across Sessions.
CREATE TABLE IF NOT EXISTS campaign.map_parties (
    map_id uuid PRIMARY KEY REFERENCES campaign.maps (id) ON DELETE CASCADE,
    node_id uuid NOT NULL REFERENCES campaign.map_nodes (id) ON DELETE CASCADE
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS map_parties_node_idx ON campaign.map_parties (node_id);

ALTER TABLE play.sessions ADD COLUMN IF NOT EXISTS world_map_id uuid;
ALTER TABLE play.sessions ADD CONSTRAINT sessions_world_map_fk FOREIGN KEY (world_map_id) REFERENCES campaign.maps (id) ON DELETE SET NULL NOT VALID;

CREATE TABLE IF NOT EXISTS play.travel_legs (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    from_name text NOT NULL,
    to_name text NOT NULL,
    pace text NOT NULL CHECK (pace IN ('slow', 'normal', 'fast')),
    distance_mi integer NOT NULL CHECK (distance_mi BETWEEN 1 AND 2000),
    minutes integer NOT NULL CHECK (minutes >= 0),
    days integer NOT NULL CHECK (days >= 0)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS travel_legs_session_idx ON play.travel_legs (session_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS travel_legs_map_idx ON play.travel_legs (map_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set', 'table_set', 'world_set', 'node_added',
    'node_removed', 'route_added', 'route_removed', 'party_placed', 'travel_leg')) NOT VALID;
