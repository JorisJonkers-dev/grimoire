-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Map's grid as it is drawn over the picture, and how many miles one cell of a world Map covers.
ALTER TABLE campaign.maps ADD COLUMN IF NOT EXISTS grid_kind text NOT NULL DEFAULT 'hexes';
ALTER TABLE campaign.maps ADD COLUMN IF NOT EXISTS grid_strength integer NOT NULL DEFAULT 20;
ALTER TABLE campaign.maps ADD COLUMN IF NOT EXISTS scale_miles double precision NOT NULL DEFAULT 6;
ALTER TABLE campaign.maps DROP CONSTRAINT IF EXISTS maps_grid_kind_check;
ALTER TABLE campaign.maps ADD CONSTRAINT maps_grid_kind_check CHECK (grid_kind IN ('hexes', 'squares', 'off') AND (kind = 'world' OR grid_kind = 'hexes')) NOT VALID;
ALTER TABLE campaign.maps DROP CONSTRAINT IF EXISTS maps_grid_strength_check;
ALTER TABLE campaign.maps ADD CONSTRAINT maps_grid_strength_check CHECK (grid_strength BETWEEN 0 AND 100) NOT VALID;
ALTER TABLE campaign.maps DROP CONSTRAINT IF EXISTS maps_scale_miles_check;
ALTER TABLE campaign.maps ADD CONSTRAINT maps_scale_miles_check CHECK (scale_miles >= 0.1 AND scale_miles <= 1000) NOT VALID;
