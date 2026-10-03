-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Rule Variant a DM authors: at a hook point it applies an Effect to whoever it happened to, or has
-- them roll on a Roll Table of the Library.
CREATE TABLE IF NOT EXISTS campaign.rule_variant_hooks (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    hook text NOT NULL CHECK (hook IN ('natural-1', 'critical', 'drop-to-0', 'rest', 'cast')),
    roll_table uuid,
    effect text CHECK (char_length(effect) BETWEEN 1 AND 80),
    created_at timestamptz NOT NULL,
    CONSTRAINT rule_variant_hooks_one_outcome_check CHECK ((roll_table IS NULL) <> (effect IS NULL))
);
CREATE INDEX IF NOT EXISTS rule_variant_hooks_campaign_idx ON campaign.rule_variant_hooks (campaign_id, created_at);

-- A roll on a Roll Table that is still open: the table, and the hook that asked for it.
ALTER TABLE play.pending_actions ADD COLUMN IF NOT EXISTS roll_table uuid;
ALTER TABLE play.pending_actions ADD COLUMN IF NOT EXISTS hook_name text;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_hook_name_check CHECK (char_length(hook_name) BETWEEN 1 AND 80) NOT VALID;
-- A roll on a table is made against no DC.
ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_dc_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_dc_check CHECK (dc BETWEEN 0 AND 40) NOT VALID;
ALTER TABLE play.pending_actions DROP CONSTRAINT IF EXISTS pending_actions_action_check;
ALTER TABLE play.pending_actions ADD CONSTRAINT pending_actions_action_check
    CHECK (action IN ('hide', 'grapple', 'shove_push', 'shove_prone', 'topple', 'concentration', 'stabilise', 'disarm', 'pick', 'force', 'influence', 'roll_table')) NOT VALID;

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
    'legendary_action', 'lair_action', 'legendary_resistance', 'checkpoint_created', 'session_rewound', 'party_split', 'party_rejoined', 'xp_awarded', 'control_assigned', 'map_found', 'map_lost', 'clock_set', 'marching_order_set', 'rule_hook_fired', 'roll_table_rolled')) NOT VALID;
