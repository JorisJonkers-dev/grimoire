-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Summons: an Effect that brings creatures in, which its caster controls and which leave when it ends.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts', 'temp_hp', 'teleport', 'forced_move',
    'dispel', 'counter', 'grant_feature', 'resource_change', 'choice', 'mode', 'branch', 'summon')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_summons (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'summon' CHECK (kind = 'summon'),
    monster_slug text NOT NULL CHECK (char_length(monster_slug) BETWEEN 1 AND 80),
    count integer NOT NULL CHECK (count BETWEEN 1 AND 20),
    shares_turn boolean NOT NULL,
    needs_command boolean NOT NULL,
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- A summoned token names the Effect that keeps it here; its Combatant names the Combatant who controls it.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS summon_effect_id uuid;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS owner_combatant_id uuid;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS commanded boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD CONSTRAINT combatants_owner_fkey
    FOREIGN KEY (owner_combatant_id) REFERENCES play.combatants (id) ON DELETE SET NULL NOT VALID;

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded')) NOT VALID;

-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount)
    VALUES ('find-familiar', 'Find Familiar', false, 'spell', 'find-familiar', 'until_dispelled', 0) ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'summon');
        INSERT INTO compendium.effect_summons (effect_id, ordinal, monster_slug, count, shares_turn, needs_command) VALUES (e, 0, 'owl', 1, true, false);
    END IF;

    e := NULL;
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug, duration_kind, duration_amount)
    VALUES ('animate-dead', 'Animate Dead', false, 'spell', 'animate-dead', 'hours', 24) ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'summon');
        INSERT INTO compendium.effect_summons (effect_id, ordinal, monster_slug, count, shares_turn, needs_command) VALUES (e, 0, 'skeleton', 1, false, true);
    END IF;
END
$$;
-- +goose StatementEnd
