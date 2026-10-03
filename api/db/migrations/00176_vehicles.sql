-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A vehicle or ship of a Campaign: its hull, the damage threshold under which a blow does nothing, and
-- the miles it covers in a day's travel when whole and fully crewed.
CREATE TABLE IF NOT EXISTS campaign.vehicles (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    kind text NOT NULL CHECK (kind IN ('land', 'water', 'air')),
    hull_hp integer NOT NULL CHECK (hull_hp >= 0),
    hull_max integer NOT NULL CHECK (hull_max BETWEEN 1 AND 10000),
    threshold integer NOT NULL CHECK (threshold BETWEEN 0 AND 100),
    miles_per_day integer NOT NULL CHECK (miles_per_day BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL,
    CONSTRAINT vehicles_hull_check CHECK (hull_hp <= hull_max)
);
CREATE INDEX IF NOT EXISTS vehicles_campaign_idx ON campaign.vehicles (campaign_id, created_at);

-- A part of a vehicle with hit points of its own; one that drives moves the vehicle.
CREATE TABLE IF NOT EXISTS campaign.vehicle_components (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES campaign.vehicles (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 19),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    hp integer NOT NULL CHECK (hp >= 0),
    hp_max integer NOT NULL CHECK (hp_max BETWEEN 1 AND 10000),
    drives boolean NOT NULL,
    CONSTRAINT vehicle_components_within_check CHECK (hp <= hp_max),
    CONSTRAINT vehicle_components_name_key UNIQUE (vehicle_id, name),
    CONSTRAINT vehicle_components_position_key UNIQUE (vehicle_id, position)
);

-- A crew station: how many crew it takes, and how many are posted there.
CREATE TABLE IF NOT EXISTS campaign.vehicle_stations (
    id uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES campaign.vehicles (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 19),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    crew integer NOT NULL CHECK (crew BETWEEN 0 AND 200),
    posted integer NOT NULL CHECK (posted >= 0),
    CONSTRAINT vehicle_stations_manned_check CHECK (posted <= crew),
    CONSTRAINT vehicle_stations_name_key UNIQUE (vehicle_id, name),
    CONSTRAINT vehicle_stations_position_key UNIQUE (vehicle_id, position)
);

-- The vehicle a Travel Leg was made aboard, by the name it had then.
ALTER TABLE play.travel_legs ADD COLUMN IF NOT EXISTS vehicle text;
ALTER TABLE play.travel_legs ADD CONSTRAINT travel_legs_vehicle_check CHECK (char_length(vehicle) BETWEEN 1 AND 80) NOT VALID;
