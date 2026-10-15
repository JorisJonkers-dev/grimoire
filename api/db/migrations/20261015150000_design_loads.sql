-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Each load of the SRD Design files, by the hash over every file, so an unchanged set is not loaded again.
CREATE TABLE IF NOT EXISTS ops.design_loads (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    design_hash text NOT NULL CHECK (design_hash ~ '^[0-9a-f]{64}$'),
    loaded_at timestamptz NOT NULL DEFAULT now()
);

-- The Effects the last load wrote, so an Effect whose file is gone is removed at the next load.
CREATE TABLE IF NOT EXISTS compendium.design_effects (
    effect_id bigint PRIMARY KEY REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE
);
