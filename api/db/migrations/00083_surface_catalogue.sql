-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Surfaces are data: the built-in ones below, and any an author adds. Becoming '' is bare ground.
CREATE TABLE IF NOT EXISTS compendium.surface_definitions (
    slug text PRIMARY KEY CHECK (slug ~ '^[a-z][a-z0-9-]{0,39}$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    cost integer NOT NULL CHECK (cost BETWEEN 1 AND 8),
    obscures text CHECK (obscures IN ('darkness', 'heavy')),
    hazard_dice text CHECK (hazard_dice ~ '^[0-9]{1,2}d[0-9]{1,3}$'),
    hazard_type text CHECK (char_length(hazard_type) BETWEEN 1 AND 20),
    every_step boolean NOT NULL DEFAULT false,
    effect_slug text CHECK (char_length(effect_slug) BETWEEN 1 AND 80),
    CHECK ((hazard_dice IS NULL) = (hazard_type IS NULL))
);

CREATE TABLE IF NOT EXISTS compendium.surface_reactions (
    surface text NOT NULL REFERENCES compendium.surface_definitions (slug) ON DELETE CASCADE,
    damage_type text NOT NULL CHECK (char_length(damage_type) BETWEEN 1 AND 20),
    becomes text NOT NULL CHECK (char_length(becomes) <= 40),
    PRIMARY KEY (surface, damage_type)
);

INSERT INTO compendium.surface_definitions (slug, name, cost, obscures, hazard_dice, hazard_type, every_step, effect_slug) VALUES
    ('fire', 'Fire', 1, NULL, '1d4', 'fire', false, NULL),
    ('grease', 'Grease', 2, NULL, NULL, NULL, false, NULL),
    ('water', 'Water', 1, NULL, NULL, NULL, false, NULL),
    ('ice', 'Ice', 2, NULL, NULL, NULL, false, NULL),
    ('web', 'Web', 2, NULL, NULL, NULL, false, NULL),
    ('electrified', 'Electrified water', 1, NULL, '1d4', 'lightning', false, NULL),
    ('fog', 'Fog', 1, 'heavy', NULL, NULL, false, NULL),
    ('darkness', 'Magical darkness', 1, 'darkness', NULL, NULL, false, NULL),
    ('stinking-cloud', 'Stinking cloud', 1, 'heavy', NULL, NULL, false, 'poisoned'),
    ('plant-growth', 'Overgrowth', 4, NULL, NULL, NULL, false, NULL),
    ('spikes', 'Spikes', 2, NULL, '2d4', 'piercing', true, NULL),
    ('mud', 'Mud', 2, NULL, NULL, NULL, false, NULL),
    ('lava', 'Lava', 1, NULL, '10d10', 'fire', false, NULL),
    ('consecrated', 'Consecrated ground', 1, NULL, NULL, NULL, false, 'bless')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO compendium.surface_reactions (surface, damage_type, becomes) VALUES
    ('fire', 'cold', ''),
    ('grease', 'fire', 'fire'),
    ('water', 'cold', 'ice'),
    ('water', 'lightning', 'electrified'),
    ('ice', 'fire', 'water'),
    ('web', 'fire', ''),
    ('plant-growth', 'fire', '')
ON CONFLICT (surface, damage_type) DO NOTHING;

-- Any catalogued Surface may now lie on a hex or be created by an Effect.
ALTER TABLE play.surfaces DROP CONSTRAINT IF EXISTS surfaces_kind_check;
ALTER TABLE play.surfaces ADD CONSTRAINT surfaces_kind_check CHECK (kind ~ '^[a-z][a-z0-9-]{0,39}$') NOT VALID;
ALTER TABLE compendium.effect_surfaces DROP CONSTRAINT IF EXISTS effect_surfaces_surface_check;
ALTER TABLE compendium.effect_surfaces ADD CONSTRAINT effect_surfaces_surface_check CHECK (surface ~ '^[a-z][a-z0-9-]{0,39}$') NOT VALID;
