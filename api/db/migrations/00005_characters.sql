-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS campaign.characters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    owner_member_id uuid NOT NULL REFERENCES campaign.members (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    ruleset text NOT NULL CHECK (ruleset IN ('srd-2024', 'srd-2014')),
    species_slug text NOT NULL,
    class_slug text NOT NULL,
    background_slug text NOT NULL,
    level integer NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 20),
    ability_method text NOT NULL CHECK (ability_method IN ('standard-array', 'point-buy', 'rolled')),
    hp_max integer NOT NULL CHECK (hp_max >= 1),
    hp_current integer NOT NULL CHECK (hp_current BETWEEN 0 AND hp_max),
    armor_slug text,
    shield boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS characters_campaign_idx ON campaign.characters (campaign_id, name);

CREATE TABLE IF NOT EXISTS campaign.character_abilities (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    base integer NOT NULL CHECK (base BETWEEN 3 AND 18),
    bonus integer NOT NULL CHECK (bonus BETWEEN 0 AND 2),
    PRIMARY KEY (character_id, ability)
);

CREATE TABLE IF NOT EXISTS campaign.character_skills (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    skill text NOT NULL,
    source text NOT NULL CHECK (source IN ('class', 'background')),
    PRIMARY KEY (character_id, skill)
);

CREATE TABLE IF NOT EXISTS campaign.character_weapons (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    weapon_slug text NOT NULL,
    ordering integer NOT NULL,
    PRIMARY KEY (character_id, weapon_slug)
);
