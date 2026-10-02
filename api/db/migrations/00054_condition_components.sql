-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The components the SRD conditions need: no actions, no movement, saving throws changed or failed,
-- close hits turned critical, and exhaustion that stacks by level.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_save_edges (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'save_edge' CHECK (kind = 'save_edge'),
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    mode text NOT NULL CHECK (mode IN ('advantage', 'disadvantage', 'fail')),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_crits (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'crit_within' CHECK (kind = 'crit_within'),
    feet integer NOT NULL CHECK (feet BETWEEN 0 AND 120),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_exhaustion (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'exhausting' CHECK (kind = 'exhausting'),
    d20_per_level integer NOT NULL CHECK (d20_per_level BETWEEN 0 AND 10),
    speed_ft_per_level integer NOT NULL CHECK (speed_ft_per_level BETWEEN 0 AND 30),
    death_at integer NOT NULL CHECK (death_at BETWEEN 0 AND 10),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- How many levels of a stacking Effect (exhaustion) a creature has.
ALTER TABLE play.active_effects ADD COLUMN IF NOT EXISTS level integer NOT NULL DEFAULT 1;
ALTER TABLE play.active_effects ADD CONSTRAINT active_effects_level_check CHECK (level BETWEEN 1 AND 10) NOT VALID;
