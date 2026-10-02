-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An Account need not have an email: Members who came in through forward-auth become Accounts without
-- one, and add it from their Account page. Grimoire is the only client and ships with this change.
-- squawk-ignore ban-drop-not-null
ALTER TABLE identity.accounts ALTER COLUMN email DROP NOT NULL;

-- Every Campaign Member becomes an Account keyed on the same subject, so their Campaigns stay theirs;
-- the external login with that subject links on its first sign-in.
INSERT INTO identity.accounts (id, subject, username, nickname, email, admin, created_at)
SELECT gen_random_uuid(), m.auth_subject, 'member-' || substr(md5(m.auth_subject), 1, 10),
    coalesce(nullif(left(trim(max(m.display_name)), 40), ''), 'Player'), NULL, false, min(m.joined_at)
FROM campaign.members m
WHERE NOT EXISTS (SELECT 1 FROM identity.accounts a WHERE a.subject = m.auth_subject)
GROUP BY m.auth_subject
ON CONFLICT DO NOTHING;

-- A Character (ADR-0010): a hero owned by an Account, with its identity and build, playable in several
-- Campaigns. Each Campaign's own progress stays on a Campaign Character, the rows of campaign.characters.
CREATE TABLE IF NOT EXISTS campaign.account_characters (
    id uuid PRIMARY KEY,
    owner_subject text NOT NULL CHECK (char_length(owner_subject) BETWEEN 1 AND 200),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    ruleset text NOT NULL CHECK (ruleset IN ('srd-2024', 'srd-2014')),
    species_slug text NOT NULL,
    class_slug text NOT NULL,
    background_slug text NOT NULL,
    backstory text NOT NULL DEFAULT '' CHECK (char_length(backstory) <= 4000),
    portrait_key text,
    portrait_type text,
    token_key text,
    token_type text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS account_characters_owner_idx ON campaign.account_characters (owner_subject);

-- Each existing Character splits into a Character and its one Campaign Character, sharing an id.
INSERT INTO campaign.account_characters (
    id, owner_subject, name, ruleset, species_slug, class_slug, background_slug,
    portrait_key, portrait_type, token_key, token_type, created_at, updated_at
)
SELECT c.id, m.auth_subject, c.name, c.ruleset, c.species_slug, c.class_slug, c.background_slug,
    c.portrait_key, c.portrait_type, c.token_key, c.token_type, c.created_at, c.updated_at
FROM campaign.characters c JOIN campaign.members m ON m.id = c.owner_member_id
ON CONFLICT (id) DO NOTHING;

ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS character_id uuid;
UPDATE campaign.characters SET character_id = id WHERE character_id IS NULL;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_character_fk
FOREIGN KEY (character_id) REFERENCES campaign.account_characters (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE campaign.characters ADD CONSTRAINT characters_character_set CHECK (character_id IS NOT NULL) NOT VALID;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS characters_one_per_campaign_idx ON campaign.characters (campaign_id, character_id);
