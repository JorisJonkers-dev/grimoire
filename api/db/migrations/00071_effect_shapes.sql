-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Choices and branches nest components: a mode or a branch is a component whose children name it as
-- their parent.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts', 'temp_hp', 'teleport', 'forced_move',
    'dispel', 'counter', 'grant_feature', 'resource_change', 'choice', 'mode', 'branch')) NOT VALID;
ALTER TABLE compendium.effect_components ADD COLUMN IF NOT EXISTS parent integer;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_parent_fkey
    FOREIGN KEY (effect_id, parent) REFERENCES compendium.effect_components (effect_id, ordinal) ON DELETE CASCADE NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_modes (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'mode' CHECK (kind = 'mode'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compendium.effect_branches (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'branch' CHECK (kind = 'branch'),
    condition text NOT NULL CHECK (condition IN ('fails_by', 'hp_at_most', 'first_each_turn', 'creature_is')),
    n integer NOT NULL CHECK (n BETWEEN 0 AND 1000),
    creature_type text NOT NULL CHECK (char_length(creature_type) <= 40),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- How long an Effect lasts, and the save that ends it early; no kind leaves the length to the DM.
ALTER TABLE compendium.effect_definitions ADD COLUMN IF NOT EXISTS duration_kind text;
ALTER TABLE compendium.effect_definitions ADD COLUMN IF NOT EXISTS duration_amount integer NOT NULL DEFAULT 0;
ALTER TABLE compendium.effect_definitions ADD COLUMN IF NOT EXISTS repeat_save text;
ALTER TABLE compendium.effect_definitions ADD CONSTRAINT effect_definitions_duration_check CHECK (
    (duration_kind IS NULL OR duration_kind IN ('instant', 'rounds', 'minutes', 'hours', 'until_dispelled', 'until_rest', 'permanent', 'end_of_next_turn'))
    AND duration_amount BETWEEN 0 AND 1000
    AND (repeat_save IS NULL OR repeat_save IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'))
) NOT VALID;

-- How an Effect's dice grow: by slot, character level, class level or a class table's column.
CREATE TABLE IF NOT EXISTS compendium.effect_scalings (
    effect_id bigint PRIMARY KEY REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE,
    axis text NOT NULL CHECK (axis IN ('slot_level', 'character_level', 'class_level', 'table_column')),
    class_slug text NOT NULL CHECK (char_length(class_slug) <= 80),
    column_name text NOT NULL CHECK (char_length(column_name) <= 80),
    base_level integer NOT NULL CHECK (base_level BETWEEN 0 AND 20),
    dice text NOT NULL CHECK (dice = '' OR dice ~ '^[0-9]{1,2}d[0-9]{1,3}$')
);

CREATE TABLE IF NOT EXISTS compendium.effect_scaling_steps (
    effect_id bigint NOT NULL REFERENCES compendium.effect_scalings (effect_id) ON DELETE CASCADE,
    at_level integer NOT NULL CHECK (at_level BETWEEN 1 AND 20),
    dice text NOT NULL CHECK (dice ~ '^[0-9]{1,2}d[0-9]{1,3}$'),
    PRIMARY KEY (effect_id, at_level)
);

-- The mode an Effect was applied in.
ALTER TABLE play.active_effects ADD COLUMN IF NOT EXISTS mode text;
ALTER TABLE play.active_effects ADD CONSTRAINT active_effects_mode_check CHECK (mode IS NULL OR char_length(mode) BETWEEN 1 AND 80) NOT VALID;

-- +goose StatementBegin
DO $$
DECLARE
    d record;
BEGIN
    FOR d IN SELECT * FROM (VALUES
        ('bless', 'minutes', 1), ('faerie-fire', 'minutes', 1), ('hunters-mark', 'hours', 1), ('grease', 'minutes', 1),
        ('spirit-guardians', 'minutes', 10), ('wall-of-fire', 'minutes', 1), ('false-life', 'hours', 1), ('dodging', 'end_of_next_turn', 0),
        ('burning-hands', 'instant', 0), ('thunderwave', 'instant', 0), ('shatter', 'instant', 0), ('fireball', 'instant', 0),
        ('lightning-bolt', 'instant', 0), ('cone-of-cold', 'instant', 0), ('misty-step', 'instant', 0), ('dispel-magic', 'instant', 0)
    ) AS v (slug, kind, amount) LOOP
        UPDATE compendium.effect_definitions SET duration_kind = d.kind, duration_amount = d.amount
        WHERE slug = d.slug AND duration_kind IS NULL;
    END LOOP;
    FOR d IN SELECT * FROM (VALUES
        ('burning-hands', 1, '1d6'), ('thunderwave', 1, '1d8'), ('shatter', 2, '1d8'), ('fireball', 3, '1d6'),
        ('lightning-bolt', 3, '1d6'), ('cone-of-cold', 5, '1d8'), ('spirit-guardians', 3, '1d8'), ('wall-of-fire', 4, '1d8')
    ) AS v (slug, base, dice) LOOP
        INSERT INTO compendium.effect_scalings (effect_id, axis, class_slug, column_name, base_level, dice)
        SELECT id, 'slot_level', '', '', d.base, d.dice FROM compendium.effect_definitions WHERE slug = d.slug
        ON CONFLICT (effect_id) DO NOTHING;
    END LOOP;
END
$$;
-- +goose StatementEnd
