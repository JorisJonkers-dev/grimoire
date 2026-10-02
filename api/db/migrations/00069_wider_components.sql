-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The wider component set: temporary hit points, teleports, forced movement, dispelling, countering,
-- granted features and Resource changes; and walls and rings as area shapes.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts', 'temp_hp', 'teleport', 'forced_move',
    'dispel', 'counter', 'grant_feature', 'resource_change')) NOT VALID;

ALTER TABLE compendium.effect_areas DROP CONSTRAINT IF EXISTS effect_areas_shape_check;
ALTER TABLE compendium.effect_areas ADD CONSTRAINT effect_areas_shape_check
    CHECK (shape IN ('cone', 'sphere', 'cube', 'cylinder', 'line', 'emanation', 'ring', 'wall')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_temp_hp (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'temp_hp' CHECK (kind = 'temp_hp'),
    amount integer NOT NULL CHECK (amount BETWEEN 1 AND 500),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compendium.effect_teleports (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'teleport' CHECK (kind = 'teleport'),
    range_ft integer NOT NULL CHECK (range_ft BETWEEN 5 AND 5280),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compendium.effect_forced_moves (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'forced_move' CHECK (kind = 'forced_move'),
    ft integer NOT NULL CHECK (ft BETWEEN 5 AND 120),
    toward boolean NOT NULL,
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compendium.effect_counters (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'counter' CHECK (kind = 'counter'),
    range_ft integer NOT NULL CHECK (range_ft BETWEEN 5 AND 5280),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compendium.effect_grants (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'grant_feature' CHECK (kind = 'grant_feature'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compendium.effect_resource_changes (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'resource_change' CHECK (kind = 'resource_change'),
    resource_slug text NOT NULL CHECK (char_length(resource_slug) BETWEEN 1 AND 80),
    delta integer NOT NULL CHECK (delta BETWEEN -20 AND 20 AND delta <> 0),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- Temporary hit points a token has on top of its hit points.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS temp_hp integer NOT NULL DEFAULT 0;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_temp_hp_check CHECK (temp_hp BETWEEN 0 AND 999) NOT VALID;

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set', 'table_set', 'world_set', 'node_added',
    'node_removed', 'route_added', 'route_removed', 'party_placed', 'travel_leg', 'zone_added',
    'zone_removed', 'zone_held', 'zone_sprung', 'perception_rolled',
    'rest_taken', 'check_scheduled', 'encounter_checked', 'encounter_resolved', 'loot_dropped',
    'item_moved', 'coins_moved', 'shop_opened',
    'shop_closed', 'item_bought', 'item_sold', 'haggle_started', 'haggled', 'stock_rolled',
    'encounter_spawned', 'hp_adjusted',
    'rest_proposed', 'rest_agreed', 'rest_started', 'hit_die_spent', 'hit_die_healed', 'rest_interrupted',
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered')) NOT VALID;

-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    -- Thunderwave pushes those who fail 10 feet away, now computed rather than handed to the DM.
    SELECT d.id INTO e FROM compendium.effect_definitions d
    JOIN compendium.effect_manual m ON m.effect_id = d.id AND m.ordinal = 2
    WHERE d.slug = 'thunderwave' AND m.instruction = 'Thunderwave: creatures that failed their save are pushed 10 feet away.';
    IF e IS NOT NULL THEN
        DELETE FROM compendium.effect_components WHERE effect_id = e AND ordinal = 2;
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 2, 'forced_move');
        INSERT INTO compendium.effect_forced_moves (effect_id, ordinal, ft, toward) VALUES (e, 2, 10, false);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('misty-step', 'Misty Step', false, 'spell', 'misty-step') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'teleport');
        INSERT INTO compendium.effect_teleports (effect_id, ordinal, range_ft) VALUES (e, 0, 30);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('false-life', 'False Life', false, 'spell', 'false-life') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'temp_hp');
        INSERT INTO compendium.effect_temp_hp (effect_id, ordinal, amount) VALUES (e, 0, 9);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('dispel-magic', 'Dispel Magic', false, 'spell', 'dispel-magic') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'dispel');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('counterspell', 'Counterspell', false, 'spell', 'counterspell') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'counter');
        INSERT INTO compendium.effect_counters (effect_id, ordinal, range_ft) VALUES (e, 0, 60);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('spirit-guardians', 'Spirit Guardians', true, 'spell', 'spirit-guardians') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'emanation', 15, 0);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'wisdom', '3d8', 'radiant', true);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('wall-of-fire', 'Wall of Fire', true, 'spell', 'wall-of-fire') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'area'), (e, 1, 'save_damage');
        INSERT INTO compendium.effect_areas (effect_id, ordinal, shape, size_ft, range_ft) VALUES (e, 0, 'wall', 60, 120);
        INSERT INTO compendium.effect_save_damage (effect_id, ordinal, ability, dice, damage_type, half) VALUES (e, 1, 'dexterity', '5d8', 'fire', true);
    END IF;

END
$$;
-- +goose StatementEnd
