-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Prep data the DM prepares ahead of play: Encounter Pools and Encounter Tables, and the Encounter
-- Checks rolled against them.
CREATE SCHEMA IF NOT EXISTS prep;

CREATE TABLE IF NOT EXISTS prep.encounter_pools (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    level_min integer NOT NULL CHECK (level_min BETWEEN 1 AND 20),
    level_max integer NOT NULL CHECK (level_max BETWEEN 1 AND 20),
    difficulty text NOT NULL CHECK (difficulty IN ('low', 'moderate', 'high')),
    updated_at timestamptz NOT NULL,
    CHECK (level_min <= level_max)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_pools_campaign_idx ON prep.encounter_pools (campaign_id, name);

CREATE TABLE IF NOT EXISTS prep.pool_members (
    pool_id uuid NOT NULL REFERENCES prep.encounter_pools (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    monster_slug text NOT NULL CHECK (char_length(monster_slug) BETWEEN 1 AND 80),
    weight integer NOT NULL CHECK (weight BETWEEN 1 AND 100),
    min_count integer NOT NULL CHECK (min_count BETWEEN 0 AND 20),
    max_count integer NOT NULL CHECK (max_count BETWEEN 1 AND 20),
    PRIMARY KEY (pool_id, ordering),
    CHECK (min_count <= max_count)
);

-- An Encounter Table belongs to a Region: a location on a world map, or everywhere when it has none.
CREATE TABLE IF NOT EXISTS prep.encounter_tables (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    region_node_id uuid REFERENCES campaign.map_nodes (id) ON DELETE SET NULL,
    chance_pct integer NOT NULL CHECK (chance_pct BETWEEN 0 AND 100),
    visibility text NOT NULL CHECK (visibility IN ('secret', 'open')),
    updated_at timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_tables_campaign_idx ON prep.encounter_tables (campaign_id, name);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_tables_region_idx ON prep.encounter_tables (region_node_id);

-- An entry is a prepared Encounter (its monsters), a draw from a Pool, or Nothing.
CREATE TABLE IF NOT EXISTS prep.table_entries (
    table_id uuid NOT NULL REFERENCES prep.encounter_tables (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    weight integer NOT NULL CHECK (weight BETWEEN 1 AND 100),
    kind text NOT NULL CHECK (kind IN ('encounter', 'pool', 'nothing')),
    label text NOT NULL CHECK (char_length(label) <= 80),
    pool_id uuid REFERENCES prep.encounter_pools (id),
    PRIMARY KEY (table_id, ordering),
    CHECK ((kind = 'pool') = (pool_id IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS table_entries_pool_idx ON prep.table_entries (pool_id);

CREATE TABLE IF NOT EXISTS prep.entry_monsters (
    table_id uuid NOT NULL,
    ordering integer NOT NULL,
    position integer NOT NULL CHECK (position >= 0),
    monster_slug text NOT NULL CHECK (char_length(monster_slug) BETWEEN 1 AND 80),
    count integer NOT NULL CHECK (count BETWEEN 1 AND 20),
    PRIMARY KEY (table_id, ordering, position),
    FOREIGN KEY (table_id, ordering) REFERENCES prep.table_entries (table_id, ordering) ON DELETE CASCADE
);

-- Snapshots of each Revision of a Pool or Table, so any of them can be restored.
CREATE TABLE IF NOT EXISTS prep.pool_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    name text NOT NULL,
    level_min integer NOT NULL,
    level_max integer NOT NULL,
    difficulty text NOT NULL
);

CREATE TABLE IF NOT EXISTS prep.pool_revision_members (
    revision_id uuid NOT NULL REFERENCES prep.pool_revisions (revision_id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    monster_slug text NOT NULL,
    weight integer NOT NULL,
    min_count integer NOT NULL,
    max_count integer NOT NULL,
    PRIMARY KEY (revision_id, ordering)
);

CREATE TABLE IF NOT EXISTS prep.table_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    name text NOT NULL,
    region_node_id uuid,
    chance_pct integer NOT NULL,
    visibility text NOT NULL
);

CREATE TABLE IF NOT EXISTS prep.table_revision_entries (
    revision_id uuid NOT NULL REFERENCES prep.table_revisions (revision_id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    weight integer NOT NULL,
    kind text NOT NULL,
    label text NOT NULL,
    pool_id uuid,
    PRIMARY KEY (revision_id, ordering)
);

CREATE TABLE IF NOT EXISTS prep.table_revision_monsters (
    revision_id uuid NOT NULL,
    ordering integer NOT NULL,
    position integer NOT NULL,
    monster_slug text NOT NULL,
    count integer NOT NULL,
    PRIMARY KEY (revision_id, ordering, position),
    FOREIGN KEY (revision_id, ordering) REFERENCES prep.table_revision_entries (revision_id, ordering) ON DELETE CASCADE
);

-- A check the DM asked for later: at the next rest or the next Travel Leg.
CREATE TABLE IF NOT EXISTS prep.scheduled_checks (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    table_id uuid NOT NULL REFERENCES prep.encounter_tables (id) ON DELETE CASCADE,
    due text NOT NULL CHECK (due IN ('next_rest', 'next_travel')),
    created_at timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS scheduled_checks_campaign_idx ON prep.scheduled_checks (campaign_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS scheduled_checks_table_idx ON prep.scheduled_checks (table_id);

-- One Encounter Check: seeded, so its draw can be replayed, and kept with what it produced.
CREATE TABLE IF NOT EXISTS prep.encounter_checks (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    session_id uuid REFERENCES play.sessions (id) ON DELETE SET NULL,
    table_id uuid REFERENCES prep.encounter_tables (id) ON DELETE SET NULL,
    table_name text NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('short_rest', 'long_rest', 'travel_leg', 'dm')),
    mode text NOT NULL CHECK (mode IN ('normal', 'force_encounter', 'pick')),
    visibility text NOT NULL CHECK (visibility IN ('secret', 'open')),
    seed bigint NOT NULL,
    chance_pct integer NOT NULL CHECK (chance_pct BETWEEN 0 AND 100),
    chance_roll integer CHECK (chance_roll BETWEEN 1 AND 100),
    roll_id uuid REFERENCES play.roll_requests (id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('pending', 'resolved')),
    outcome text CHECK (outcome IN ('encounter', 'nothing')),
    entry_label text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    CHECK ((status = 'resolved') = (outcome IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_checks_campaign_idx ON prep.encounter_checks (campaign_id, created_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_checks_session_idx ON prep.encounter_checks (session_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_checks_table_idx ON prep.encounter_checks (table_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS encounter_checks_roll_idx ON prep.encounter_checks (roll_id);

CREATE TABLE IF NOT EXISTS prep.check_monsters (
    check_id uuid NOT NULL REFERENCES prep.encounter_checks (id) ON DELETE CASCADE,
    position integer NOT NULL CHECK (position >= 0),
    monster_slug text NOT NULL,
    count integer NOT NULL CHECK (count BETWEEN 1 AND 100),
    PRIMARY KEY (check_id, position)
);

ALTER TABLE campaign.revisions DROP CONSTRAINT IF EXISTS revisions_entity_type_check;
ALTER TABLE campaign.revisions ADD CONSTRAINT revisions_entity_type_check
    CHECK (entity_type IN ('npc', 'encounter_pool', 'encounter_table', 'encounter_check')) NOT VALID;

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
    'rest_taken', 'check_scheduled', 'encounter_checked', 'encounter_resolved')) NOT VALID;
