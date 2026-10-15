-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Manual Effect's instruction runs to 500 letters, as the homebrew builders always let it.
ALTER TABLE compendium.effect_manual ADD CONSTRAINT effect_manual_instruction_length CHECK (char_length(instruction) BETWEEN 1 AND 500) NOT VALID;
ALTER TABLE compendium.effect_manual DROP CONSTRAINT IF EXISTS effect_manual_instruction_check;
