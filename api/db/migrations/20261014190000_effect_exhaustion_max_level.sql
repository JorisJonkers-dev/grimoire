-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The highest level a stacking Part rises to; 0 has no cap.
ALTER TABLE compendium.effect_exhaustion ADD COLUMN IF NOT EXISTS max_level integer NOT NULL DEFAULT 0;
ALTER TABLE compendium.effect_exhaustion ADD CONSTRAINT effect_exhaustion_max_level_check CHECK (max_level BETWEEN 0 AND 10) NOT VALID;
