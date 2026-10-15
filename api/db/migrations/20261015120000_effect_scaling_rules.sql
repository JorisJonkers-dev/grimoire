-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Scaling grows one scalable value of an Effect, many per Effect: a field of one Part, or of the
-- Activation when part is NULL, by a linear rule, a table of steps, a formula of the level, or a column
-- of a class table.
CREATE TABLE IF NOT EXISTS compendium.effect_scaling_rules (
    effect_id bigint NOT NULL REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 99),
    part integer,
    field text NOT NULL CHECK (field ~ '^[a-z][a-z0-9_]{0,39}$'),
    axis text NOT NULL CHECK (axis IN ('slot_level', 'character_level', 'class_level', 'table_column', 'charges')),
    class_slug text NOT NULL CHECK (char_length(class_slug) <= 80),
    rule text NOT NULL CHECK (rule IN ('linear', 'table', 'formula', 'column')),
    base_level integer NOT NULL CHECK (base_level BETWEEN 0 AND 20),
    per_dice text CHECK (per_dice ~ '^[0-9]{1,2}d[0-9]{1,3}$'),
    per_number integer CHECK (per_number BETWEEN -10000 AND 10000),
    formula text CHECK (char_length(formula) BETWEEN 1 AND 60 AND formula ~ '^[0-9level+*() -]+$'),
    column_name text NOT NULL CHECK (char_length(column_name) <= 80),
    PRIMARY KEY (effect_id, position),
    FOREIGN KEY (effect_id, part) REFERENCES compendium.effect_components (effect_id, ordinal) ON DELETE CASCADE,
    CHECK ((rule = 'linear') = (num_nonnulls(per_dice, per_number) = 1)),
    CHECK (num_nonnulls(per_dice, per_number) <= 1),
    CHECK ((rule = 'formula') = (formula IS NOT NULL)),
    CHECK ((rule = 'column') = (axis = 'table_column'))
);

-- The steps of a table: the value from a level on, typed as the field it fills.
CREATE TABLE IF NOT EXISTS compendium.effect_scaling_rule_steps (
    effect_id bigint NOT NULL,
    position integer NOT NULL,
    at_level integer NOT NULL CHECK (at_level BETWEEN 1 AND 20),
    value_dice text CHECK (value_dice ~ '^[0-9]{1,2}d[0-9]{1,3}$'),
    value_number integer CHECK (value_number BETWEEN -10000 AND 10000),
    value_flag boolean,
    value_text text CHECK (char_length(value_text) BETWEEN 1 AND 40),
    PRIMARY KEY (effect_id, position, at_level),
    FOREIGN KEY (effect_id, position) REFERENCES compendium.effect_scaling_rules (effect_id, position) ON DELETE CASCADE,
    CHECK (num_nonnulls(value_dice, value_number, value_flag, value_text) = 1)
);

-- Backfill: the one Scaling an Effect had grew the damage of its area, which is the dice of its last
-- save_damage Part. compendium.effect_scalings and its steps stay as they are for older binaries.
INSERT INTO compendium.effect_scaling_rules (effect_id, position, part, field, axis, class_slug, rule, base_level, per_dice, column_name)
SELECT s.effect_id, 0, p.ordinal, 'dice', s.axis, s.class_slug,
    CASE s.axis WHEN 'slot_level' THEN 'linear' WHEN 'table_column' THEN 'column' ELSE 'table' END,
    s.base_level, CASE WHEN s.axis = 'slot_level' THEN s.dice END, s.column_name
FROM compendium.effect_scalings s
JOIN LATERAL (
    SELECT max(c.ordinal) AS ordinal FROM compendium.effect_components c
    WHERE c.effect_id = s.effect_id AND c.kind = 'save_damage' AND c.parent IS NULL
) p ON p.ordinal IS NOT NULL
WHERE NOT (s.axis = 'slot_level' AND s.dice = '')
ON CONFLICT (effect_id, position) DO NOTHING;

INSERT INTO compendium.effect_scaling_rule_steps (effect_id, position, at_level, value_dice)
SELECT st.effect_id, 0, st.at_level, st.dice
FROM compendium.effect_scaling_steps st
JOIN compendium.effect_scaling_rules r ON r.effect_id = st.effect_id AND r.position = 0 AND r.rule = 'table'
ON CONFLICT (effect_id, position, at_level) DO NOTHING;
