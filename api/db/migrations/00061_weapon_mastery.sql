-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Weapon Mastery: a speed penalty that does not stack (Slow), an advantage only the Effect's source
-- enjoys (Vex), the mastery an attack carries, Topple's saving throw, and Cleave's second attack.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_speed_penalties (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'speed_penalty' CHECK (kind = 'speed_penalty'),
    ft integer NOT NULL CHECK (ft BETWEEN 0 AND 120),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

ALTER TABLE compendium.effect_edges ADD COLUMN IF NOT EXISTS source_only boolean NOT NULL DEFAULT false;

ALTER TABLE play.token_attacks ADD COLUMN IF NOT EXISTS mastery text;
ALTER TABLE play.token_attacks ADD CONSTRAINT token_attacks_mastery_check
    CHECK (mastery IN ('cleave', 'graze', 'nick', 'push', 'sap', 'slow', 'topple', 'vex')) NOT VALID;

ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_action_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_action_check
    CHECK (action IN ('hide', 'grapple', 'shove_push', 'shove_prone', 'topple')) NOT VALID;

ALTER TABLE play.attacks ADD COLUMN IF NOT EXISTS cleave boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS cleave_from uuid;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS cleaved boolean NOT NULL DEFAULT false;

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used')) NOT VALID;

-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('sapped', 'Sapped', false, 'item_property', 'sap') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, false, false, 'any');
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('slowed', 'Slowed', false, 'item_property', 'slow') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'speed_penalty');
        INSERT INTO compendium.effect_speed_penalties (effect_id, ordinal, ft) VALUES (e, 0, 10);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('vexed', 'Vexed', false, 'item_property', 'vex') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach, source_only) VALUES (e, 0, true, true, 'any', true);
    END IF;
END
$$;
-- +goose StatementEnd
