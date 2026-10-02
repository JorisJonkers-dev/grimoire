-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Which learned spells a Character has prepared, and which sit in a wizard's spellbook.
ALTER TABLE campaign.character_spells ADD COLUMN IF NOT EXISTS prepared boolean NOT NULL DEFAULT true;
ALTER TABLE campaign.character_spells ADD COLUMN IF NOT EXISTS spellbook boolean NOT NULL DEFAULT false;
UPDATE campaign.character_spells cs SET spellbook = true
WHERE cs.class_slug = 'wizard' AND EXISTS (SELECT 1 FROM compendium.spells s WHERE s.slug = cs.spell_slug AND s.level > 0);

-- A Character may change its prepared spells: after a long rest, a level, or before it first prepares.
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS can_prepare boolean NOT NULL DEFAULT true;

-- The time of day on a Campaign's Game Clock, in minutes after midnight of its game day.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS game_minute integer NOT NULL DEFAULT 0;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_game_minute_check CHECK (game_minute BETWEEN 0 AND 1439) NOT VALID;

-- Spells a class or subclass always has prepared from a level on; they do not count against the limit.
CREATE TABLE IF NOT EXISTS compendium.always_prepared (
    owner_kind text NOT NULL CHECK (owner_kind IN ('class', 'subclass')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80),
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    spell_slug text NOT NULL CHECK (char_length(spell_slug) BETWEEN 1 AND 80),
    PRIMARY KEY (owner_kind, owner_slug, spell_slug)
);

INSERT INTO compendium.always_prepared (owner_kind, owner_slug, level, spell_slug) VALUES
    ('class', 'druid', 1, 'speak-with-animals'),
    ('class', 'paladin', 2, 'divine-smite'),
    ('class', 'ranger', 1, 'hunters-mark'),
    ('subclass', 'life-domain', 3, 'aid'),
    ('subclass', 'life-domain', 3, 'bless'),
    ('subclass', 'life-domain', 3, 'cure-wounds'),
    ('subclass', 'life-domain', 3, 'lesser-restoration'),
    ('subclass', 'life-domain', 5, 'mass-healing-word'),
    ('subclass', 'life-domain', 5, 'revivify'),
    ('subclass', 'life-domain', 7, 'aura-of-life'),
    ('subclass', 'life-domain', 7, 'death-ward'),
    ('subclass', 'life-domain', 9, 'greater-restoration'),
    ('subclass', 'life-domain', 9, 'mass-cure-wounds'),
    ('subclass', 'oath-of-devotion', 3, 'protection-from-evil-and-good'),
    ('subclass', 'oath-of-devotion', 3, 'shield-of-faith'),
    ('subclass', 'oath-of-devotion', 5, 'aid'),
    ('subclass', 'oath-of-devotion', 5, 'zone-of-truth'),
    ('subclass', 'oath-of-devotion', 9, 'beacon-of-hope'),
    ('subclass', 'oath-of-devotion', 9, 'dispel-magic'),
    ('subclass', 'oath-of-devotion', 13, 'freedom-of-movement'),
    ('subclass', 'oath-of-devotion', 13, 'guardian-of-faith'),
    ('subclass', 'oath-of-devotion', 17, 'commune'),
    ('subclass', 'oath-of-devotion', 17, 'flame-strike'),
    ('subclass', 'fiend-patron', 3, 'burning-hands'),
    ('subclass', 'fiend-patron', 3, 'command'),
    ('subclass', 'fiend-patron', 3, 'scorching-ray'),
    ('subclass', 'fiend-patron', 3, 'suggestion'),
    ('subclass', 'fiend-patron', 5, 'fireball'),
    ('subclass', 'fiend-patron', 5, 'stinking-cloud'),
    ('subclass', 'fiend-patron', 7, 'fire-shield'),
    ('subclass', 'fiend-patron', 7, 'wall-of-fire'),
    ('subclass', 'fiend-patron', 9, 'geas'),
    ('subclass', 'fiend-patron', 9, 'insect-plague'),
    ('subclass', 'draconic-sorcery', 3, 'alter-self'),
    ('subclass', 'draconic-sorcery', 3, 'chromatic-orb'),
    ('subclass', 'draconic-sorcery', 3, 'command'),
    ('subclass', 'draconic-sorcery', 3, 'dragons-breath'),
    ('subclass', 'draconic-sorcery', 5, 'fear'),
    ('subclass', 'draconic-sorcery', 5, 'fly'),
    ('subclass', 'draconic-sorcery', 7, 'arcane-eye'),
    ('subclass', 'draconic-sorcery', 7, 'charm-monster'),
    ('subclass', 'draconic-sorcery', 9, 'legend-lore'),
    ('subclass', 'draconic-sorcery', 9, 'summon-dragon')
ON CONFLICT DO NOTHING;
