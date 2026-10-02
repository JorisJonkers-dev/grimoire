-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Forms: an Effect that overlays its bearer's stat block with a creature's until it reverts.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts', 'temp_hp', 'teleport', 'forced_move',
    'dispel', 'counter', 'grant_feature', 'resource_change', 'choice', 'mode', 'branch', 'summon', 'form')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_forms (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'form' CHECK (kind = 'form'),
    monster_slug text NOT NULL CHECK (char_length(monster_slug) <= 80),
    temp_hp integer NOT NULL CHECK (temp_hp BETWEEN 0 AND 999),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- The form a token has taken: the Effect keeping it, and the creature's statistics laid over the token's own.
CREATE TABLE IF NOT EXISTS play.token_forms (
    token_id uuid PRIMARY KEY REFERENCES play.tokens (id) ON DELETE CASCADE,
    effect_id uuid NOT NULL,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    stats jsonb NOT NULL CHECK (pg_column_size(stats) <= 65536)
);

-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount)
    VALUES ('polymorph', 'Polymorph', true, 'spell', 'polymorph', 'hours', 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'form');
        INSERT INTO compendium.effect_forms (effect_id, ordinal, monster_slug, temp_hp) VALUES (e, 0, '', 0);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount)
    VALUES ('wild-shape', 'Wild Shape', false, 'feature', 'wild-shape', 'hours', 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'form');
        INSERT INTO compendium.effect_forms (effect_id, ordinal, monster_slug, temp_hp) VALUES (e, 0, '', 0);
    END IF;
END
$$;
-- +goose StatementEnd
