-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Character's rebuildable choices at one moment: what a retrain proposes, or what a Revision kept.
CREATE TABLE IF NOT EXISTS campaign.character_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id uuid NOT NULL,
    species_slug text NOT NULL CHECK (char_length(species_slug) BETWEEN 1 AND 80),
    background_slug text NOT NULL CHECK (char_length(background_slug) BETWEEN 1 AND 80),
    ability_method text NOT NULL CHECK (ability_method IN ('standard-array', 'point-buy', 'rolled')),
    created_at timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS character_snapshots_character_idx ON campaign.character_snapshots (character_id);

CREATE TABLE IF NOT EXISTS campaign.character_snapshot_abilities (
    snapshot_id uuid NOT NULL REFERENCES campaign.character_snapshots (id) ON DELETE CASCADE,
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    base integer NOT NULL CHECK (base BETWEEN 3 AND 18),
    bonus integer NOT NULL CHECK (bonus BETWEEN 0 AND 2),
    increase integer NOT NULL CHECK (increase BETWEEN 0 AND 20),
    PRIMARY KEY (snapshot_id, ability)
);

CREATE TABLE IF NOT EXISTS campaign.character_snapshot_skills (
    snapshot_id uuid NOT NULL REFERENCES campaign.character_snapshots (id) ON DELETE CASCADE,
    skill text NOT NULL CHECK (char_length(skill) BETWEEN 1 AND 40),
    PRIMARY KEY (snapshot_id, skill)
);

CREATE TABLE IF NOT EXISTS campaign.character_snapshot_picks (
    snapshot_id uuid NOT NULL REFERENCES campaign.character_snapshots (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    choice text NOT NULL CHECK (char_length(choice) BETWEEN 1 AND 80),
    value text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 80),
    PRIMARY KEY (snapshot_id, level, choice, value)
);

-- A player's request to rebuild a Campaign Character, which the DM approves or declines.
CREATE TABLE IF NOT EXISTS campaign.retrains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    proposed_id uuid NOT NULL REFERENCES campaign.character_snapshots (id),
    status text NOT NULL CHECK (status IN ('pending', 'approved', 'declined')),
    reason text NOT NULL CHECK (char_length(reason) <= 500),
    requested_by text NOT NULL CHECK (char_length(requested_by) BETWEEN 1 AND 80),
    decided_by text NOT NULL DEFAULT '' CHECK (char_length(decided_by) <= 80),
    created_at timestamptz NOT NULL,
    decided_at timestamptz,
    CHECK ((status = 'pending') = (decided_at IS NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS retrains_character_idx ON campaign.retrains (character_id, created_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS retrains_one_pending_idx ON campaign.retrains (character_id) WHERE status = 'pending';

-- A Character's Revision keeps the build an approved retrain replaced.
CREATE TABLE IF NOT EXISTS campaign.character_revisions (
    revision_id uuid PRIMARY KEY REFERENCES campaign.revisions (id) ON DELETE CASCADE,
    snapshot_id uuid NOT NULL REFERENCES campaign.character_snapshots (id),
    retrain_id uuid REFERENCES campaign.retrains (id) ON DELETE SET NULL
);

ALTER TABLE campaign.revisions DROP CONSTRAINT IF EXISTS revisions_entity_type_check;
ALTER TABLE campaign.revisions ADD CONSTRAINT revisions_entity_type_check
    CHECK (entity_type IN ('npc', 'encounter_pool', 'encounter_table', 'encounter_check', 'loot_table', 'settlement', 'shop', 'character')) NOT VALID;
