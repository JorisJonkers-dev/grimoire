-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Bright light and dim light beyond it, shed from where an Effect lands.
CREATE TABLE IF NOT EXISTS compendium.effect_lights (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'light' CHECK (kind = 'light'),
    bright_ft integer NOT NULL CHECK (bright_ft BETWEEN 5 AND 120),
    dim_ft integer NOT NULL CHECK (dim_ft BETWEEN 0 AND 120),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

INSERT INTO compendium.effect_kinds (slug) VALUES ('light') ON CONFLICT (slug) DO NOTHING;
