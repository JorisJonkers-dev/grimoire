-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Effects as data: what a spell, feature, item property, condition, monster action, surface, rule
-- variant, trap or roll-table result does, as typed components the rules engine resolves.
CREATE TABLE IF NOT EXISTS compendium.effect_definitions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND char_length(slug) <= 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    concentration boolean NOT NULL,
    owner_kind text NOT NULL CHECK (owner_kind IN ('spell', 'feature', 'item_property', 'condition', 'monster_action',
        'surface', 'rule_variant', 'trap', 'roll_table_result')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80)
);

-- One row per component, in order; the typed row for it lives in exactly the table its kind names.
CREATE TABLE IF NOT EXISTS compendium.effect_components (
    effect_id bigint NOT NULL REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    kind text NOT NULL CHECK (kind IN ('bonus_die', 'edge', 'extra_damage', 'move_cost', 'manual', 'area', 'save_damage',
        'save_condition', 'create_surface')),
    PRIMARY KEY (effect_id, ordinal),
    UNIQUE (effect_id, ordinal, kind)
);

CREATE TABLE IF NOT EXISTS compendium.effect_bonus_dice (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'bonus_die' CHECK (kind = 'bonus_die'),
    dice text NOT NULL CHECK (dice ~ '^[0-9]+d[0-9]+$'),
    on_attacks boolean NOT NULL,
    on_saves boolean NOT NULL,
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE,
    CHECK (on_attacks OR on_saves)
);

CREATE TABLE IF NOT EXISTS compendium.effect_edges (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'edge' CHECK (kind = 'edge'),
    against boolean NOT NULL,
    advantage boolean NOT NULL,
    reach text NOT NULL CHECK (reach IN ('any', 'within_five', 'beyond_five')),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_extra_damage (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'extra_damage' CHECK (kind = 'extra_damage'),
    dice text NOT NULL CHECK (dice ~ '^[0-9]+d[0-9]+$'),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_move_costs (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'move_cost' CHECK (kind = 'move_cost'),
    multiplier integer NOT NULL CHECK (multiplier BETWEEN 1 AND 4),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_manual (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'manual' CHECK (kind = 'manual'),
    instruction text NOT NULL CHECK (char_length(instruction) BETWEEN 1 AND 300),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_areas (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'area' CHECK (kind = 'area'),
    shape text NOT NULL CHECK (shape IN ('cone', 'sphere', 'cube', 'cylinder', 'line', 'emanation')),
    size_ft integer NOT NULL CHECK (size_ft BETWEEN 5 AND 500),
    range_ft integer NOT NULL CHECK (range_ft BETWEEN 0 AND 5280),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_save_damage (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'save_damage' CHECK (kind = 'save_damage'),
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    dice text NOT NULL CHECK (dice ~ '^[0-9]+d[0-9]+$'),
    damage_type text NOT NULL CHECK (char_length(damage_type) BETWEEN 1 AND 30),
    half boolean NOT NULL,
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_save_conditions (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'save_condition' CHECK (kind = 'save_condition'),
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    condition_slug text NOT NULL CHECK (char_length(condition_slug) BETWEEN 1 AND 80),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_surfaces (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'create_surface' CHECK (kind = 'create_surface'),
    surface text NOT NULL CHECK (surface IN ('fire', 'grease', 'water', 'ice', 'web', 'electrified')),
    rounds integer NOT NULL CHECK (rounds BETWEEN 1 AND 1000),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
