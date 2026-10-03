-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What makes a token legendary in play: its legendary and lair actions, Legendary Resistance, mythic
-- phases and damage threshold, with what it has left.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS legend jsonb;

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded', 'visibility_set', 'object_placed', 'object_removed', 'object_toggled', 'object_damaged', 'object_found', 'object_unlocked', 'trap_disarmed', 'trap_sprung', 'jumped', 'thrown', 'sneak_started', 'sneak_ended', 'stealth_rolled', 'party_noticed', 'exploration_started', 'exploration_turn', 'exploration_ended', 'loot_claimed', 'loot_settled', 'trade_made',
    'legendary_action', 'lair_action', 'legendary_resistance')) NOT VALID;
