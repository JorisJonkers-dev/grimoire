-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS campaign.maps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    image_key text NOT NULL,
    image_type text NOT NULL,
    width_px integer NOT NULL CHECK (width_px > 0),
    height_px integer NOT NULL CHECK (height_px > 0),
    hex_size_px double precision NOT NULL CHECK (hex_size_px >= 8),
    origin_x double precision NOT NULL,
    origin_y double precision NOT NULL,
    ambient text NOT NULL DEFAULT 'bright' CHECK (ambient IN ('bright', 'dim', 'dark')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS maps_campaign_idx ON campaign.maps (campaign_id, name);

CREATE TABLE IF NOT EXISTS campaign.map_walls (
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    PRIMARY KEY (map_id, q, r)
);

CREATE TABLE IF NOT EXISTS campaign.map_lights (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    bright_ft integer NOT NULL CHECK (bright_ft BETWEEN 0 AND 600),
    dim_ft integer NOT NULL CHECK (dim_ft BETWEEN 0 AND 600)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS map_lights_map_idx ON campaign.map_lights (map_id);

-- What the party has ever seen on a map; it stays seen across Sessions.
CREATE TABLE IF NOT EXISTS campaign.map_reveals (
    map_id uuid NOT NULL REFERENCES campaign.maps (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    PRIMARY KEY (map_id, q, r)
);

ALTER TABLE play.sessions ADD COLUMN IF NOT EXISTS map_id uuid;
ALTER TABLE play.sessions ADD CONSTRAINT sessions_map_fk FOREIGN KEY (map_id) REFERENCES campaign.maps (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS darkvision_ft integer NOT NULL DEFAULT 0;

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set')) NOT VALID;

CREATE TABLE IF NOT EXISTS play.action_hex_events (
    action_id uuid NOT NULL REFERENCES play.actions (id) ON DELETE CASCADE,
    q integer NOT NULL,
    r integer NOT NULL,
    PRIMARY KEY (action_id, q, r)
);
