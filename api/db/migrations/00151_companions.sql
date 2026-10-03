-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Companion or hireling is an ally who travels with the party: a creature with a name of its own,
-- run by a Player or by the DM, who may take a share of the XP. It keeps its hit points between Sessions.
CREATE TABLE IF NOT EXISTS campaign.companions (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    kind text NOT NULL CHECK (kind IN ('companion', 'hireling')),
    monster_slug text NOT NULL CHECK (char_length(monster_slug) BETWEEN 1 AND 120),
    controller_member_id uuid REFERENCES campaign.members (id) ON DELETE SET NULL,
    shares_xp boolean NOT NULL DEFAULT false,
    hp_current integer CHECK (hp_current IS NULL OR hp_current >= 0),
    notes text NOT NULL DEFAULT '' CHECK (char_length(notes) <= 2000),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS companions_campaign_idx ON campaign.companions (campaign_id);

-- The token a Companion is on the map as.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS companion_id uuid;
ALTER TABLE play.tokens DROP CONSTRAINT IF EXISTS tokens_companion_fk;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_companion_fk FOREIGN KEY (companion_id) REFERENCES campaign.companions (id) ON DELETE SET NULL NOT VALID;

-- XP a Campaign Character has earned, and what each award gave to whom.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS xp integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.characters DROP CONSTRAINT IF EXISTS characters_xp_check;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_xp_check CHECK (xp >= 0) NOT VALID;
CREATE TABLE IF NOT EXISTS play.xp_awards (
    action_id uuid NOT NULL REFERENCES play.actions (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    amount integer NOT NULL CHECK (amount >= 0),
    PRIMARY KEY (action_id, character_id)
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
    'encounter_spawned', 'hp_adjusted',
    'rest_proposed', 'rest_agreed', 'rest_started', 'hit_die_spent', 'hit_die_healed', 'rest_interrupted',
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded', 'visibility_set', 'object_placed', 'object_removed', 'object_toggled', 'object_damaged', 'object_found', 'object_unlocked', 'trap_disarmed', 'trap_sprung', 'jumped', 'thrown', 'sneak_started', 'sneak_ended', 'stealth_rolled', 'party_noticed', 'exploration_started', 'exploration_turn', 'exploration_ended', 'loot_claimed', 'loot_settled', 'trade_made',
    'legendary_action', 'lair_action', 'legendary_resistance', 'checkpoint_created', 'session_rewound', 'party_split', 'party_rejoined', 'xp_awarded', 'control_assigned')) NOT VALID;
