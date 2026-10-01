-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What classes, species, backgrounds and feats grant, as data the rules engine resolves. Owners are
-- named by kind and slug so homebrew owners fit the same rows.

-- A pool of uses: Rage, Focus Points, Channel Divinity.
CREATE TABLE IF NOT EXISTS compendium.resources (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (char_length(slug) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    owner_kind text NOT NULL CHECK (owner_kind IN ('class', 'subclass', 'species', 'background', 'feat')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80),
    basis text NOT NULL CHECK (basis IN ('table', 'class_level', 'ability', 'proficiency')),
    multiplier integer NOT NULL CHECK (multiplier BETWEEN 0 AND 20),
    ability text CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    from_level integer NOT NULL CHECK (from_level BETWEEN 1 AND 20),
    CHECK ((basis = 'ability') = (ability IS NOT NULL)),
    CHECK ((basis = 'class_level') = (multiplier > 0))
);

-- A table Resource's maximum from a level on.
CREATE TABLE IF NOT EXISTS compendium.resource_maxima (
    resource_id bigint NOT NULL REFERENCES compendium.resources (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    maximum integer NOT NULL CHECK (maximum BETWEEN 0 AND 100),
    PRIMARY KEY (resource_id, level)
);

-- The die one use rolls, from a level on.
CREATE TABLE IF NOT EXISTS compendium.resource_dice (
    resource_id bigint NOT NULL REFERENCES compendium.resources (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    die text NOT NULL CHECK (die IN ('d4', 'd6', 'd8', 'd10', 'd12', 'd20')),
    PRIMARY KEY (resource_id, level)
);

-- How uses come back: on an event, from a level, some (amount) or all (no amount).
CREATE TABLE IF NOT EXISTS compendium.resource_recharges (
    resource_id bigint NOT NULL REFERENCES compendium.resources (id) ON DELETE CASCADE,
    event text NOT NULL CHECK (event IN ('short_rest', 'long_rest', 'dawn', 'initiative', 'roll')),
    from_level integer NOT NULL CHECK (from_level BETWEEN 1 AND 20),
    amount integer CHECK (amount BETWEEN 1 AND 100),
    roll_at_least integer CHECK (roll_at_least BETWEEN 2 AND 6),
    PRIMARY KEY (resource_id, event, from_level),
    CHECK ((event = 'roll') = (roll_at_least IS NOT NULL))
);

-- A value that grows with level: Sneak Attack dice, the Martial Arts die, Rage damage.
CREATE TABLE IF NOT EXISTS compendium.scales (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (char_length(slug) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    owner_kind text NOT NULL CHECK (owner_kind IN ('class', 'subclass', 'species', 'background', 'feat')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80)
);

CREATE TABLE IF NOT EXISTS compendium.scale_steps (
    scale_id bigint NOT NULL REFERENCES compendium.scales (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    value text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 20),
    PRIMARY KEY (scale_id, level)
);

-- A pick made at a level: a Fighting Style, a subclass, two Expertise skills.
CREATE TABLE IF NOT EXISTS compendium.choices (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_kind text NOT NULL CHECK (owner_kind IN ('class', 'subclass', 'species', 'background', 'feat')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80),
    slug text NOT NULL CHECK (char_length(slug) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    count integer NOT NULL CHECK (count BETWEEN 1 AND 10),
    pool text NOT NULL CHECK (pool IN ('feat_category', 'subclass', 'skill', 'expertise', 'weapon', 'listed')),
    pool_from text NOT NULL CHECK (char_length(pool_from) BETWEEN 0 AND 80),
    UNIQUE (owner_kind, owner_slug, slug, level)
);

-- What must hold before something is taken. Rows sharing a group are alternatives; every group must hold.
CREATE TABLE IF NOT EXISTS compendium.prerequisites (
    owner_kind text NOT NULL CHECK (owner_kind IN ('class', 'subclass', 'species', 'background', 'feat')),
    owner_slug text NOT NULL CHECK (char_length(owner_slug) BETWEEN 1 AND 80),
    group_no integer NOT NULL CHECK (group_no BETWEEN 0 AND 20),
    ordinal integer NOT NULL CHECK (ordinal BETWEEN 0 AND 20),
    kind text NOT NULL CHECK (kind IN ('level', 'ability', 'spellcasting', 'feat', 'feature')),
    ability text CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    minimum integer NOT NULL CHECK (minimum BETWEEN 0 AND 30),
    ref_slug text CHECK (char_length(ref_slug) BETWEEN 1 AND 80),
    PRIMARY KEY (owner_kind, owner_slug, group_no, ordinal),
    CHECK ((kind = 'ability') = (ability IS NOT NULL)),
    CHECK ((kind IN ('feat', 'feature')) = (ref_slug IS NOT NULL))
);
