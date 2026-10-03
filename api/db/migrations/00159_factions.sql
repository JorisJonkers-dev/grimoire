-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Faction of a Campaign, with how it regards the party: a score only the DM sees, read as a tier.
CREATE TABLE IF NOT EXISTS campaign.factions (
    id uuid PRIMARY KEY,
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    archetype text NOT NULL CHECK (char_length(archetype) <= 60),
    goals text NOT NULL CHECK (char_length(goals) <= 2000),
    territory text NOT NULL CHECK (char_length(territory) <= 2000),
    notes text NOT NULL CHECK (char_length(notes) <= 2000),
    score integer NOT NULL CHECK (score BETWEEN -100 AND 100),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS factions_campaign_idx ON campaign.factions (campaign_id);

-- A Personal Standing: one Character's own Standing with a Faction, used instead of the party's.
CREATE TABLE IF NOT EXISTS campaign.personal_standings (
    faction_id uuid NOT NULL REFERENCES campaign.factions (id) ON DELETE CASCADE,
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    score integer NOT NULL CHECK (score BETWEEN -100 AND 100),
    PRIMARY KEY (faction_id, character_id)
);
CREATE INDEX IF NOT EXISTS personal_standings_character_idx ON campaign.personal_standings (character_id);

-- A Standing Change: suggested, then confirmed or dismissed by the DM. character_id is null for the party.
CREATE TABLE IF NOT EXISTS campaign.standing_changes (
    id uuid PRIMARY KEY,
    faction_id uuid NOT NULL REFERENCES campaign.factions (id) ON DELETE CASCADE,
    character_id uuid REFERENCES campaign.characters (id) ON DELETE CASCADE,
    delta integer NOT NULL CHECK (delta BETWEEN -100 AND 100 AND delta <> 0),
    reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
    share_reason boolean NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'confirmed', 'dismissed')),
    origin text NOT NULL CHECK (origin IN ('ui', 'mcp', 'generator', 'system')),
    client text NOT NULL CHECK (char_length(client) <= 120),
    proposed_by text NOT NULL CHECK (char_length(proposed_by) <= 200),
    created_at timestamptz NOT NULL,
    decided_at timestamptz
);
CREATE INDEX IF NOT EXISTS standing_changes_faction_idx ON campaign.standing_changes (faction_id, created_at);
CREATE INDEX IF NOT EXISTS standing_changes_character_idx ON campaign.standing_changes (character_id);
