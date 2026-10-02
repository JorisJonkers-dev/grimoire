-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Character's call on an item in a loot pile: need or greed, with the d20 rolled for it. The item is a
-- plain stack's slug or an Item Instance's id.
CREATE TABLE IF NOT EXISTS campaign.loot_claims (
    container_id uuid NOT NULL REFERENCES campaign.containers (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    item text NOT NULL CHECK (length(item) BETWEEN 1 AND 80),
    choice text NOT NULL CHECK (choice IN ('need', 'greed')),
    roll integer NOT NULL CHECK (roll BETWEEN 1 AND 20),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (container_id, character_id, item)
);
CREATE INDEX IF NOT EXISTS loot_claims_character_idx ON campaign.loot_claims (character_id);

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded', 'visibility_set', 'object_placed', 'object_removed', 'object_toggled', 'object_damaged', 'object_found', 'object_unlocked', 'trap_disarmed', 'trap_sprung', 'jumped', 'thrown', 'sneak_started', 'sneak_ended', 'stealth_rolled', 'party_noticed', 'exploration_started', 'exploration_turn', 'exploration_ended', 'loot_claimed', 'loot_settled')) NOT VALID;
