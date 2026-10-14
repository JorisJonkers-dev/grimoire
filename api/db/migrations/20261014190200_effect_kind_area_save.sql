-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The save every creature in an area makes when what a failure brings sits in a branch.
CREATE TABLE IF NOT EXISTS compendium.effect_area_saves (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'area_save' CHECK (kind = 'area_save'),
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

INSERT INTO compendium.effect_kinds (slug) VALUES ('area_save') ON CONFLICT (slug) DO NOTHING;
