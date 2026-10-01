-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The Effects the rules engine shipped with, now as data. An Effect already present is left alone, so
-- a DM's or an import's later edits are never overwritten.
-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('bless', 'Bless', true, 'spell', 'bless') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'bonus_die');
        INSERT INTO compendium.effect_bonus_dice (effect_id, ordinal, dice, on_attacks, on_saves) VALUES (e, 0, '1d4', true, true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('faerie-fire', 'Faerie Fire', true, 'spell', 'faerie-fire') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, true, true, 'any');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('hunters-mark', 'Hunter''s Mark', true, 'spell', 'hunters-mark') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'extra_damage');
        INSERT INTO compendium.effect_extra_damage (effect_id, ordinal, dice) VALUES (e, 0, '1d6');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('fireball', 'Fireball', false, 'spell', 'fireball') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'sphere', 20, 150);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'dexterity', '8d6', 'fire', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('burning-hands', 'Burning Hands', false, 'spell', 'burning-hands') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'cone', 15, 0);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'dexterity', '3d6', 'fire', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('lightning-bolt', 'Lightning Bolt', false, 'spell', 'lightning-bolt') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'line', 100, 0);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'dexterity', '8d6', 'lightning', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('cone-of-cold', 'Cone of Cold', false, 'spell', 'cone-of-cold') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'cone', 60, 0);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'constitution', '8d8', 'cold', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('shatter', 'Shatter', false, 'spell', 'shatter') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'sphere', 10, 60);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'constitution', '3d8', 'thunder', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('thunderwave', 'Thunderwave', false, 'spell', 'thunderwave') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage'), (e, 2, 'manual');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'cube', 15, 0);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'constitution', '2d8', 'thunder', true);
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 2, 'Thunderwave: creatures that failed their save are pushed 10 feet away.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('grease', 'Grease', false, 'spell', 'grease') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'create_surface'), (e, 2, 'save_condition');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'cylinder', 5, 60);
        INSERT INTO compendium.effect_surfaces (effect_id, ordinal, surface, rounds) VALUES (e, 1, 'grease', 10);
        INSERT INTO compendium.effect_save_conditions (effect_id, ordinal, ability, condition_slug) VALUES (e, 2, 'dexterity', 'prone');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('prone', 'Prone', false, 'condition', 'prone') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'edge'), (e, 2, 'edge'), (e, 3, 'move_cost');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES
            (e, 0, false, false, 'any'), (e, 1, true, true, 'within_five'), (e, 2, true, false, 'beyond_five');
        INSERT INTO compendium.effect_move_costs (effect_id, ordinal, multiplier) VALUES (e, 3, 2);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('poisoned', 'Poisoned', false, 'condition', 'poisoned') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'manual');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, false, false, 'any');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 1, 'Poisoned: ability checks are made with disadvantage.');
    END IF;
END
$$;
-- +goose StatementEnd
