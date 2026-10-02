-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Every SRD 5.2 condition as an Effect, beside Prone and Poisoned. An Effect already present is left
-- alone, so a DM's later edits are never overwritten.
-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('blinded', 'Blinded', false, 'condition', 'blinded') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'edge'), (e, 2, 'manual');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, true, true, 'any');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 1, false, false, 'any');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 2, 'Blinded: can''t see, and fails any ability check that needs sight.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('charmed', 'Charmed', false, 'condition', 'charmed') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'manual');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 0, 'Charmed: can''t attack the charmer or target them with harmful effects; the charmer has advantage on social checks against them.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('deafened', 'Deafened', false, 'condition', 'deafened') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'manual');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 0, 'Deafened: can''t hear, and fails any ability check that needs hearing.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('exhaustion', 'Exhaustion', false, 'condition', 'exhaustion') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'exhausting');
        INSERT INTO compendium.effect_exhaustion (effect_id, ordinal, d20_per_level, speed_ft_per_level, death_at) VALUES (e, 0, 2, 5, 6);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('frightened', 'Frightened', false, 'condition', 'frightened') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'manual');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, false, false, 'any');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 1, 'Frightened: can''t willingly move closer to the source of its fear, and the disadvantage holds only while it can see that source.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('grappled', 'Grappled', false, 'condition', 'grappled') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'immobile'), (e, 1, 'manual');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 1, 'Grappled: has disadvantage on attacks against anyone but the grappler, who can drag it along at half speed.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('incapacitated', 'Incapacitated', false, 'condition', 'incapacitated') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'incapacitated'), (e, 1, 'manual');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 1, 'Incapacitated: can''t speak, and rolls Initiative with disadvantage.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('invisible', 'Invisible', false, 'condition', 'invisible') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, true, false, 'any');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 1, false, true, 'any');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('paralyzed', 'Paralyzed', false, 'condition', 'paralyzed') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'incapacitated'), (e, 1, 'immobile'), (e, 2, 'save_edge'), (e, 3, 'save_edge'), (e, 4, 'edge'), (e, 5, 'crit_within');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 2, 'strength', 'fail');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 3, 'dexterity', 'fail');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 4, true, true, 'any');
        INSERT INTO compendium.effect_crits (effect_id, ordinal, feet) VALUES (e, 5, 5);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('petrified', 'Petrified', false, 'condition', 'petrified') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'incapacitated'), (e, 1, 'immobile'), (e, 2, 'save_edge'), (e, 3, 'save_edge'), (e, 4, 'edge'), (e, 5, 'manual');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 2, 'strength', 'fail');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 3, 'dexterity', 'fail');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 4, true, true, 'any');
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 5, 'Petrified: turned to stone with what it wears and carries; it resists all damage and is immune to poison.');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('restrained', 'Restrained', false, 'condition', 'restrained') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'immobile'), (e, 1, 'edge'), (e, 2, 'edge'), (e, 3, 'save_edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 1, true, true, 'any');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 2, false, false, 'any');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 3, 'dexterity', 'disadvantage');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('stunned', 'Stunned', false, 'condition', 'stunned') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'incapacitated'), (e, 1, 'save_edge'), (e, 2, 'save_edge'), (e, 3, 'edge');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 1, 'strength', 'fail');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 2, 'dexterity', 'fail');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 3, true, true, 'any');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('unconscious', 'Unconscious', false, 'condition', 'unconscious') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'incapacitated'), (e, 1, 'immobile'), (e, 2, 'save_edge'), (e, 3, 'save_edge'), (e, 4, 'edge'), (e, 5, 'crit_within'), (e, 6, 'manual');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 2, 'strength', 'fail');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 3, 'dexterity', 'fail');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 4, true, true, 'any');
        INSERT INTO compendium.effect_crits (effect_id, ordinal, feet) VALUES (e, 5, 5);
        INSERT INTO compendium.effect_manual (effect_id, ordinal, instruction) VALUES (e, 6, 'Unconscious: drops what it holds, falls Prone and knows nothing of its surroundings.');
    END IF;

END
$$;
-- +goose StatementEnd
