-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Creatures one spawn_encounter placed, so undoing it removes them together.
CREATE TABLE IF NOT EXISTS play.action_spawn_events (
    action_id uuid NOT NULL REFERENCES play.actions (id) ON DELETE CASCADE,
    token_id uuid NOT NULL,
    label text NOT NULL,
    PRIMARY KEY (action_id, token_id)
);

-- The Effect an effect_applied put on, so undoing it ends that Effect.
CREATE TABLE IF NOT EXISTS play.action_effect_events (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    effect_id uuid NOT NULL
);

-- An undo and the Action it reverted; each Action is undone at most once.
CREATE TABLE IF NOT EXISTS play.action_undos (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    undoes_action_id uuid NOT NULL UNIQUE REFERENCES play.actions (id) ON DELETE CASCADE
);

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
    'encounter_spawned', 'hp_adjusted')) NOT VALID;
