-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The levels a Campaign Character has in each class once it has levelled up; position 0 is the class it
-- started in. A Character without rows has all its levels in its starting class.
CREATE TABLE IF NOT EXISTS campaign.character_classes (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    class_slug text NOT NULL CHECK (char_length(class_slug) BETWEEN 1 AND 80),
    subclass_slug text CHECK (char_length(subclass_slug) BETWEEN 1 AND 80),
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    position integer NOT NULL CHECK (position BETWEEN 0 AND 20),
    PRIMARY KEY (character_id, class_slug)
);

-- What a Character picked on reaching a level: a Fighting Style, a feat, Expertise skills.
CREATE TABLE IF NOT EXISTS campaign.character_picks (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    choice text NOT NULL CHECK (char_length(choice) BETWEEN 1 AND 80),
    value text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 80),
    PRIMARY KEY (character_id, level, choice, value)
);

-- The cantrips and spells a Character learned, by the class it learned them through.
CREATE TABLE IF NOT EXISTS campaign.character_spells (
    character_id uuid NOT NULL REFERENCES campaign.characters (id) ON DELETE CASCADE,
    class_slug text NOT NULL CHECK (char_length(class_slug) BETWEEN 1 AND 80),
    spell_slug text NOT NULL CHECK (char_length(spell_slug) BETWEEN 1 AND 80),
    learned_level integer NOT NULL CHECK (learned_level BETWEEN 1 AND 20),
    PRIMARY KEY (character_id, class_slug, spell_slug)
);

-- Ability Score Improvements, kept apart from the build so it still checks against its background.
ALTER TABLE campaign.character_abilities ADD COLUMN IF NOT EXISTS increase integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.character_abilities ADD CONSTRAINT character_abilities_increase_check CHECK (increase BETWEEN 0 AND 20) NOT VALID;

-- A DM holding level-ups: long rests stop unlocking the next level; the DM grants levels instead.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS hold_level_ups boolean NOT NULL DEFAULT false;
