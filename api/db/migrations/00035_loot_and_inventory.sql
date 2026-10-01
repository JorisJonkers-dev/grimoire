-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Loot Table: rolled a number of times over weighted entries of items, coins, other tables or nothing.
CREATE TABLE IF NOT EXISTS prep.loot_tables (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    rolls integer NOT NULL CHECK (rolls BETWEEN 1 AND 10),
    updated_at timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS loot_tables_campaign_idx ON prep.loot_tables (campaign_id, name);

CREATE TABLE IF NOT EXISTS prep.loot_entries (
    table_id uuid NOT NULL REFERENCES prep.loot_tables (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    weight integer NOT NULL CHECK (weight BETWEEN 1 AND 100),
    kind text NOT NULL CHECK (kind IN ('item', 'currency', 'table', 'nothing')),
    item_slug text CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    coin text CHECK (coin IN ('cp', 'sp', 'ep', 'gp', 'pp')),
    amount text NOT NULL CHECK (char_length(amount) <= 20),
    nested_table_id uuid REFERENCES prep.loot_tables (id),
    PRIMARY KEY (table_id, ordering),
    CHECK ((kind = 'item') = (item_slug IS NOT NULL)),
    CHECK ((kind = 'currency') = (coin IS NOT NULL)),
    CHECK ((kind = 'table') = (nested_table_id IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS loot_entries_nested_idx ON prep.loot_entries (nested_table_id);

CREATE TABLE IF NOT EXISTS prep.loot_table_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    name text NOT NULL,
    rolls integer NOT NULL
);

CREATE TABLE IF NOT EXISTS prep.loot_revision_entries (
    revision_id uuid NOT NULL REFERENCES prep.loot_table_revisions (revision_id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    weight integer NOT NULL,
    kind text NOT NULL,
    item_slug text,
    coin text,
    amount text NOT NULL,
    nested_table_id uuid,
    PRIMARY KEY (revision_id, ordering)
);

-- Where items and coins are: a Character's Inventory, the Party Stash, or a drop of loot.
CREATE TABLE IF NOT EXISTS campaign.containers (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('character', 'party_stash', 'loot_drop')),
    character_id uuid REFERENCES campaign.characters (id) ON DELETE CASCADE,
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    created_at timestamptz NOT NULL,
    CHECK ((kind = 'character') = (character_id IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS containers_campaign_idx ON campaign.containers (campaign_id, created_at);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS containers_character_idx ON campaign.containers (character_id);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS containers_stash_idx ON campaign.containers (campaign_id) WHERE kind = 'party_stash';

CREATE TABLE IF NOT EXISTS campaign.container_items (
    container_id uuid NOT NULL REFERENCES campaign.containers (id) ON DELETE CASCADE,
    item_slug text NOT NULL CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100000),
    PRIMARY KEY (container_id, item_slug)
);

CREATE TABLE IF NOT EXISTS campaign.container_coins (
    container_id uuid NOT NULL REFERENCES campaign.containers (id) ON DELETE CASCADE,
    coin text NOT NULL CHECK (coin IN ('cp', 'sp', 'ep', 'gp', 'pp')),
    amount integer NOT NULL CHECK (amount BETWEEN 1 AND 10000000),
    PRIMARY KEY (container_id, coin)
);

-- What a drop or transfer moved, for the Action Log.
CREATE TABLE IF NOT EXISTS play.action_item_events (
    action_id uuid NOT NULL REFERENCES play.actions (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    from_label text NOT NULL,
    to_label text NOT NULL,
    item_slug text,
    coin text,
    count integer NOT NULL CHECK (count >= 1),
    PRIMARY KEY (action_id, position)
);

ALTER TABLE campaign.revisions DROP CONSTRAINT IF EXISTS revisions_entity_type_check;
ALTER TABLE campaign.revisions ADD CONSTRAINT revisions_entity_type_check
    CHECK (entity_type IN ('npc', 'encounter_pool', 'encounter_table', 'encounter_check', 'loot_table')) NOT VALID;

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
    'item_moved', 'coins_moved')) NOT VALID;
