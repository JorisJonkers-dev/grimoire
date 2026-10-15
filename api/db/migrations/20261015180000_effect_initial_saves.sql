-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The saving throw each target makes as an Effect is activated; the Effect lands only on those who fail.
-- A DC of 0 is the caster's spell save DC. No row asks for none.
CREATE TABLE IF NOT EXISTS compendium.effect_initial_saves (
    effect_id bigint PRIMARY KEY REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE,
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    dc integer NOT NULL CHECK (dc BETWEEN 0 AND 30)
);

-- An Effect waiting on its target's initial save has not landed yet; the save that settles it is marked.
ALTER TABLE play.active_effects ADD COLUMN IF NOT EXISTS pending boolean NOT NULL DEFAULT false;
