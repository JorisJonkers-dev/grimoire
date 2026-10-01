-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What a Character carries between Sessions: the Hit Dice spent, the uses of each Resource spent, and
-- whether a finished Long Rest has unlocked a level-up.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS hit_dice_spent integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_hit_dice_spent_check CHECK (hit_dice_spent BETWEEN 0 AND 20) NOT VALID;
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS level_up_ready boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS campaign.character_resources (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    resource_slug text NOT NULL CHECK (char_length(resource_slug) BETWEEN 1 AND 80),
    used integer NOT NULL CHECK (used BETWEEN 0 AND 100),
    PRIMARY KEY (character_id, resource_slug)
);

-- The optional rule that a Long Rest costs each resting Character one day of Rations.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS rest_supplies boolean NOT NULL DEFAULT false;

-- A rest the party proposed, who agreed, who is resting, and each rester's Hit Die roll still out.
CREATE TABLE IF NOT EXISTS play.rests (
    session_id uuid PRIMARY KEY REFERENCES play.sessions (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('short', 'long')),
    status text NOT NULL CHECK (status IN ('proposed', 'resting')),
    proposed_by uuid NOT NULL REFERENCES campaign.members (id) ON DELETE CASCADE,
    dm_agreed boolean NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS rests_proposed_by_idx ON play.rests (proposed_by);

CREATE TABLE IF NOT EXISTS play.rest_agreements (
    session_id uuid NOT NULL REFERENCES play.rests (session_id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES campaign.members (id) ON DELETE CASCADE,
    PRIMARY KEY (session_id, member_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS rest_agreements_member_idx ON play.rest_agreements (member_id);

CREATE TABLE IF NOT EXISTS play.rest_resters (
    session_id uuid NOT NULL REFERENCES play.rests (session_id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    token_id uuid NOT NULL,
    roll_id uuid REFERENCES play.roll_requests (id) ON DELETE SET NULL,
    PRIMARY KEY (session_id, character_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS rest_resters_character_idx ON play.rest_resters (character_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS rest_resters_roll_idx ON play.rest_resters (roll_id);

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
    'rest_proposed', 'rest_agreed', 'rest_started', 'hit_die_spent', 'hit_die_healed', 'rest_interrupted')) NOT VALID;
