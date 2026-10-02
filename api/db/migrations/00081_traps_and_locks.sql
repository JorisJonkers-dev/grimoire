-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Traps and locks on Map Objects.
ALTER TABLE campaign.map_objects DROP CONSTRAINT IF EXISTS map_objects_kind_check;
ALTER TABLE campaign.map_objects ADD CONSTRAINT map_objects_kind_check
    CHECK (kind IN ('door', 'lever', 'chest', 'barrel', 'curtain', 'destructible', 'trap')) NOT VALID;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS detect_dc integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS disarm_dc integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS trigger_ft integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS armed boolean NOT NULL DEFAULT false;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS locked boolean NOT NULL DEFAULT false;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS lock_dc integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.map_objects ADD COLUMN IF NOT EXISTS key_slug text;
ALTER TABLE campaign.map_objects ADD CONSTRAINT map_objects_trap_lock_check CHECK (
    detect_dc BETWEEN 0 AND 40 AND disarm_dc BETWEEN 0 AND 40 AND trigger_ft BETWEEN 0 AND 60 AND lock_dc BETWEEN 0 AND 40
    AND (key_slug IS NULL OR char_length(key_slug) BETWEEN 1 AND 80)
) NOT VALID;

-- Disarming, picking and forcing are checks against a Map Object.
ALTER TABLE play.pending_actions ADD COLUMN IF NOT EXISTS object_id uuid;
ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_target_check CHECK (
    (action IN ('hide', 'disarm', 'pick', 'force')) = (target_token_id IS NULL)
    AND (action IN ('disarm', 'pick', 'force')) = (object_id IS NOT NULL)
) NOT VALID;
ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_action_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_action_check
    CHECK (action IN ('hide', 'grapple', 'shove_push', 'shove_prone', 'topple', 'concentration', 'stabilise', 'disarm', 'pick', 'force')) NOT VALID;

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded', 'visibility_set', 'object_placed', 'object_removed', 'object_toggled', 'object_damaged', 'object_found', 'object_unlocked', 'trap_disarmed', 'trap_sprung')) NOT VALID;
