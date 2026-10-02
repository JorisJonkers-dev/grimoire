-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Heroic Inspiration: a Campaign Character has it or not.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS heroic_inspiration boolean NOT NULL DEFAULT false;

-- What grants Heroic Inspiration on its own, and when: the human's Resourceful trait on a long rest.
CREATE TABLE IF NOT EXISTS compendium.inspiration_grants (
    owner_kind text NOT NULL CHECK (owner_kind IN ('species', 'background', 'feat')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80),
    on_event text NOT NULL CHECK (on_event IN ('long_rest')),
    PRIMARY KEY (owner_kind, owner_slug, on_event)
);
INSERT INTO compendium.inspiration_grants (owner_kind, owner_slug, on_event) VALUES ('species', 'human', 'long_rest')
ON CONFLICT DO NOTHING;

-- A roll whose roller holds Heroic Inspiration waits, once every die is set, for the roller to keep it
-- or spend Inspiration to reroll a die; a rerolled request resolves at once.
ALTER TABLE play.roll_requests ADD COLUMN IF NOT EXISTS choosing boolean NOT NULL DEFAULT false;
ALTER TABLE play.roll_requests ADD COLUMN IF NOT EXISTS rerolled boolean NOT NULL DEFAULT false;
