-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A split party plays as groups: the Session it split from, and a Session of its own for each group
-- that went elsewhere, with its own map, fog and fight. A group names itself and points at the Session
-- it left; the Session the others stayed in says which group the Table Display follows.
ALTER TABLE play.sessions ADD COLUMN IF NOT EXISTS parent_session_id uuid;
ALTER TABLE play.sessions ADD COLUMN IF NOT EXISTS group_name text NOT NULL DEFAULT '';
ALTER TABLE play.sessions ADD COLUMN IF NOT EXISTS table_session_id uuid;
ALTER TABLE play.sessions DROP CONSTRAINT IF EXISTS sessions_parent_fk;
ALTER TABLE play.sessions ADD CONSTRAINT sessions_parent_fk FOREIGN KEY (parent_session_id) REFERENCES play.sessions (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE play.sessions DROP CONSTRAINT IF EXISTS sessions_table_fk;
ALTER TABLE play.sessions ADD CONSTRAINT sessions_table_fk FOREIGN KEY (table_session_id) REFERENCES play.sessions (id) ON DELETE SET NULL NOT VALID;
ALTER TABLE play.sessions DROP CONSTRAINT IF EXISTS sessions_group_check;
ALTER TABLE play.sessions ADD CONSTRAINT sessions_group_check CHECK (char_length(group_name) <= 40 AND (parent_session_id IS NULL OR parent_session_id <> id)) NOT VALID;
-- A Campaign has a handful of Sessions: the index is made in the blink the migration takes.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS sessions_parent_idx ON play.sessions (parent_session_id) WHERE parent_session_id IS NOT NULL;

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
    'legendary_action', 'lair_action', 'legendary_resistance', 'checkpoint_created', 'session_rewound', 'party_split', 'party_rejoined')) NOT VALID;
