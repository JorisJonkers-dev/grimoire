-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Checkpoint is a point in a live Session the DM can rewind to: one they named, or the start of a
-- round. It keeps the Session's rows as they were, by table, so a rewind puts them back as a whole.
CREATE TABLE IF NOT EXISTS play.checkpoints (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    kind text NOT NULL CHECK (kind IN ('named', 'round')),
    round integer NOT NULL CHECK (round >= 0),
    action_seq bigint NOT NULL CHECK (action_seq >= 0),
    state jsonb NOT NULL CHECK (jsonb_typeof(state) = 'object'),
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS checkpoints_session_idx ON play.checkpoints (session_id, action_seq);

-- A rewind takes back every Action after the Checkpoint it went to: none of them can be undone again.
CREATE TABLE IF NOT EXISTS play.rewinds (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    to_action_seq bigint NOT NULL CHECK (to_action_seq >= 0)
);
CREATE INDEX IF NOT EXISTS rewinds_session_idx ON play.rewinds (session_id);

-- A Campaign can be played without undo: no undo, no Checkpoints, no rewind.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS no_undo boolean NOT NULL DEFAULT false;

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
    'legendary_action', 'lair_action', 'legendary_resistance', 'checkpoint_created', 'session_rewound')) NOT VALID;
