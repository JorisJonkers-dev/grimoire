-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS compendium.documents (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key text NOT NULL UNIQUE,
    title text NOT NULL,
    ruleset_year integer NOT NULL CHECK (ruleset_year BETWEEN 2000 AND 2100),
    precedence integer NOT NULL,
    license text NOT NULL,
    attribution text NOT NULL,
    url text NOT NULL
);

CREATE TABLE IF NOT EXISTS compendium.ability_scores (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL
);

INSERT INTO compendium.ability_scores (slug, name) VALUES
    ('strength', 'Strength'), ('dexterity', 'Dexterity'), ('constitution', 'Constitution'),
    ('intelligence', 'Intelligence'), ('wisdom', 'Wisdom'), ('charisma', 'Charisma')
ON CONFLICT (slug) DO NOTHING;

CREATE TABLE IF NOT EXISTS compendium.magic_schools (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL
);

CREATE TABLE IF NOT EXISTS compendium.damage_types (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL
);

CREATE TABLE IF NOT EXISTS compendium.spells (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    level integer NOT NULL CHECK (level BETWEEN 0 AND 9),
    school_id bigint NOT NULL REFERENCES compendium.magic_schools (id),
    casting_time text NOT NULL,
    range_text text NOT NULL,
    range_feet integer CHECK (range_feet >= 0),
    requires_verbal boolean NOT NULL,
    requires_somatic boolean NOT NULL,
    requires_material boolean NOT NULL,
    material_text text,
    ritual boolean NOT NULL,
    concentration boolean NOT NULL,
    duration text NOT NULL,
    description text NOT NULL,
    higher_level text,
    save_ability_id bigint REFERENCES compendium.ability_scores (id),
    attack_roll boolean NOT NULL,
    damage_roll text,
    UNIQUE (slug, document_id)
);

-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS spells_level_school_idx ON compendium.spells (level, school_id);
-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS spells_name_trgm_idx ON compendium.spells USING gin (name gin_trgm_ops);
-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS spells_document_idx ON compendium.spells (document_id);

CREATE TABLE IF NOT EXISTS compendium.spell_classes (
    spell_id bigint NOT NULL REFERENCES compendium.spells (id) ON DELETE CASCADE,
    class_slug text NOT NULL CHECK (class_slug ~ '^[a-z][a-z-]*$'),
    PRIMARY KEY (spell_id, class_slug)
);

-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS spell_classes_class_idx ON compendium.spell_classes (class_slug);

CREATE TABLE IF NOT EXISTS compendium.spell_damage_types (
    spell_id bigint NOT NULL REFERENCES compendium.spells (id) ON DELETE CASCADE,
    damage_type_id bigint NOT NULL REFERENCES compendium.damage_types (id),
    PRIMARY KEY (spell_id, damage_type_id)
);

-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS spell_damage_types_type_idx ON compendium.spell_damage_types (damage_type_id);

CREATE TABLE IF NOT EXISTS compendium.spell_scaling (
    spell_id bigint NOT NULL REFERENCES compendium.spells (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('slot', 'character')),
    at_level integer NOT NULL CHECK (at_level BETWEEN 1 AND 20),
    damage_roll text NOT NULL,
    PRIMARY KEY (spell_id, kind, at_level)
);

CREATE TABLE IF NOT EXISTS compendium.conditions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    UNIQUE (slug, document_id)
);

-- squawk-ignore require-concurrent-index-creation -- table is created in this migration and is empty
CREATE INDEX IF NOT EXISTS conditions_document_idx ON compendium.conditions (document_id);

CREATE TABLE IF NOT EXISTS ops.compendium_imports (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    snapshot_hash text NOT NULL,
    imported_at timestamptz NOT NULL DEFAULT now()
);
