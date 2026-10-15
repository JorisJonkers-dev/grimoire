-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The slot an area spell waiting on its rolls was cast with, so it resolves with the same Scaling; NULL
-- casts it as written.
ALTER TABLE play.area_casts ADD COLUMN IF NOT EXISTS slot_level integer;
ALTER TABLE play.area_casts ADD CONSTRAINT area_casts_slot_level_check CHECK (slot_level BETWEEN 1 AND 9) NOT VALID;
