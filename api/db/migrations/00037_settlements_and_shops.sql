-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The in-game day: long rests and Travel Legs move it on, and Shops restock by it.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS game_day integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_game_day_check CHECK (game_day BETWEEN 0 AND 1000000) NOT VALID;

CREATE TABLE IF NOT EXISTS prep.settlements (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    size text NOT NULL CHECK (size IN ('hamlet', 'village', 'town', 'city')),
    wealth text NOT NULL CHECK (wealth IN ('poor', 'modest', 'comfortable', 'wealthy')),
    location_id uuid REFERENCES campaign.map_nodes (id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS settlements_campaign_idx ON prep.settlements (campaign_id, name);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS settlements_location_idx ON prep.settlements (location_id);

-- A Shop: its trade, owner, markup, how hard it haggles, the Loot Table its Stock comes from and when it restocks.
CREATE TABLE IF NOT EXISTS prep.shops (
    id uuid PRIMARY KEY,
    settlement_id uuid NOT NULL REFERENCES prep.settlements (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    kind text NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 40),
    owner_npc_id uuid REFERENCES campaign.npcs (id) ON DELETE SET NULL,
    markup_pct integer NOT NULL CHECK (markup_pct BETWEEN 0 AND 300),
    haggle_dc integer NOT NULL CHECK (haggle_dc BETWEEN 5 AND 30),
    haggle_pct integer NOT NULL CHECK (haggle_pct BETWEEN 0 AND 50),
    loot_table_id uuid REFERENCES prep.loot_tables (id) ON DELETE SET NULL,
    restock text NOT NULL CHECK (restock IN ('never', 'long_rest', 'days')),
    restock_days integer CHECK (restock_days BETWEEN 1 AND 365),
    stocked_day integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL,
    CHECK ((restock = 'days') = (restock_days IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS shops_settlement_idx ON prep.shops (settlement_id, name);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS shops_owner_idx ON prep.shops (owner_npc_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS shops_loot_idx ON prep.shops (loot_table_id);

CREATE TABLE IF NOT EXISTS prep.shop_stock (
    shop_id uuid NOT NULL REFERENCES prep.shops (id) ON DELETE CASCADE,
    item_slug text NOT NULL CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100000),
    price_cp integer NOT NULL CHECK (price_cp BETWEEN 1 AND 2000000000),
    PRIMARY KEY (shop_id, item_slug)
);

CREATE TABLE IF NOT EXISTS prep.settlement_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    name text NOT NULL,
    size text NOT NULL,
    wealth text NOT NULL,
    location_id uuid
);

CREATE TABLE IF NOT EXISTS prep.shop_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    settlement_id uuid NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    owner_npc_id uuid,
    markup_pct integer NOT NULL,
    haggle_dc integer NOT NULL,
    haggle_pct integer NOT NULL,
    loot_table_id uuid,
    restock text NOT NULL,
    restock_days integer,
    stocked_day integer NOT NULL
);

CREATE TABLE IF NOT EXISTS prep.shop_revision_stock (
    revision_id uuid NOT NULL REFERENCES prep.shop_revisions (revision_id) ON DELETE CASCADE,
    item_slug text NOT NULL,
    quantity integer NOT NULL,
    price_cp integer NOT NULL,
    PRIMARY KEY (revision_id, item_slug)
);

-- The Shop open in a Session, and each Character's haggling there: pending while its roll is out.
CREATE TABLE IF NOT EXISTS play.session_shops (
    session_id uuid PRIMARY KEY REFERENCES play.sessions (id) ON DELETE CASCADE,
    shop_id uuid NOT NULL REFERENCES prep.shops (id) ON DELETE CASCADE
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS session_shops_shop_idx ON play.session_shops (shop_id);

CREATE TABLE IF NOT EXISTS play.haggles (
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    shop_id uuid NOT NULL REFERENCES prep.shops (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    roll_id uuid REFERENCES play.roll_requests (id) ON DELETE SET NULL,
    adjust_pct integer CHECK (adjust_pct BETWEEN -50 AND 50),
    PRIMARY KEY (session_id, shop_id, character_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS haggles_shop_idx ON play.haggles (shop_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS haggles_character_idx ON play.haggles (character_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS haggles_roll_idx ON play.haggles (roll_id);

ALTER TABLE campaign.revisions DROP CONSTRAINT IF EXISTS revisions_entity_type_check;
ALTER TABLE campaign.revisions ADD CONSTRAINT revisions_entity_type_check
    CHECK (entity_type IN ('npc', 'encounter_pool', 'encounter_table', 'encounter_check', 'loot_table', 'settlement', 'shop')) NOT VALID;

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
    'shop_closed', 'item_bought', 'item_sold', 'haggle_started', 'haggled', 'stock_rolled')) NOT VALID;
