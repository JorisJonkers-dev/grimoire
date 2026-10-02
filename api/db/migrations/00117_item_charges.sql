-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- How many charges a magic item holds and what it regains, and when (SRD 5.2 item descriptions). A long
-- rest counts as passing a dawn.
CREATE TABLE IF NOT EXISTS compendium.item_charges (
    item_slug text PRIMARY KEY CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    max_charges integer NOT NULL CHECK (max_charges BETWEEN 1 AND 100),
    regain_dice integer NOT NULL CHECK (regain_dice BETWEEN 0 AND 10),
    regain_faces integer NOT NULL CHECK (regain_faces IN (0, 3, 4, 6, 8, 10, 12, 20)),
    regain_bonus integer NOT NULL CHECK (regain_bonus BETWEEN 0 AND 100),
    recharge_on text NOT NULL CHECK (recharge_on IN ('dawn', 'long_rest', 'short_rest'))
);

INSERT INTO compendium.item_charges (item_slug, max_charges, regain_dice, regain_faces, regain_bonus, recharge_on) VALUES
    ('cube-of-force', 10, 1, 6, 0, 'dawn'),
    ('cubic-gate', 3, 1, 3, 0, 'dawn'),
    ('dragon-orb', 7, 1, 4, 3, 'dawn'),
    ('gem-of-seeing', 3, 1, 3, 0, 'dawn'),
    ('hammer-of-thunderbolts-maul', 5, 1, 4, 1, 'dawn'),
    ('hammer-of-thunderbolts-warhammer', 5, 1, 4, 1, 'dawn'),
    ('helm-of-teleportation', 3, 1, 3, 0, 'dawn'),
    ('mace-of-terror', 3, 1, 3, 0, 'dawn'),
    ('medallion-of-thoughts', 5, 1, 4, 0, 'dawn'),
    ('ring-of-animal-influence', 3, 1, 3, 0, 'dawn'),
    ('ring-of-elemental-command', 5, 1, 4, 1, 'dawn'),
    ('ring-of-evasion', 3, 1, 3, 0, 'dawn'),
    ('ring-of-shooting-stars', 6, 1, 6, 0, 'dawn'),
    ('ring-of-the-ram', 3, 1, 3, 0, 'dawn'),
    ('robe-of-scintillating-colors', 3, 1, 3, 0, 'dawn'),
    ('staff-of-charming', 10, 1, 8, 2, 'dawn'),
    ('staff-of-fire', 10, 1, 6, 4, 'dawn'),
    ('staff-of-frost', 10, 1, 6, 4, 'dawn'),
    ('staff-of-healing', 10, 1, 6, 4, 'dawn'),
    ('staff-of-power', 20, 2, 8, 4, 'dawn'),
    ('staff-of-striking', 10, 1, 6, 4, 'dawn'),
    ('staff-of-swarming-insects', 10, 1, 6, 4, 'dawn'),
    ('staff-of-the-magi', 50, 4, 6, 2, 'dawn'),
    ('staff-of-the-woodlands', 6, 1, 6, 0, 'dawn'),
    ('staff-of-withering', 3, 1, 3, 0, 'dawn'),
    ('trident-of-fish-command', 3, 1, 3, 0, 'dawn'),
    ('wand-of-binding', 7, 1, 6, 1, 'dawn'),
    ('wand-of-enemy-detection', 7, 1, 6, 1, 'dawn'),
    ('wand-of-fireballs', 7, 1, 6, 1, 'dawn'),
    ('wand-of-lightning-bolts', 7, 1, 6, 1, 'dawn'),
    ('wand-of-magic-detection', 3, 1, 3, 0, 'dawn'),
    ('wand-of-magic-missiles', 7, 1, 6, 1, 'dawn'),
    ('wand-of-paralysis', 7, 1, 6, 1, 'dawn'),
    ('wand-of-polymorph', 7, 1, 6, 1, 'dawn'),
    ('wand-of-secrets', 3, 1, 3, 0, 'dawn'),
    ('wand-of-web', 7, 1, 6, 1, 'dawn'),
    ('wand-of-wonder', 7, 1, 6, 1, 'dawn')
ON CONFLICT DO NOTHING;
