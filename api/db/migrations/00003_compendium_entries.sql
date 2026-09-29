-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE TABLE IF NOT EXISTS compendium.classes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    parent_slug text,
    hit_die integer CHECK (hit_die IN (6, 8, 10, 12)),
    caster_type text NOT NULL DEFAULT 'none',
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.class_saving_throws (
    class_id bigint NOT NULL REFERENCES compendium.classes (id) ON DELETE CASCADE,
    ability_id bigint NOT NULL REFERENCES compendium.ability_scores (id),
    PRIMARY KEY (class_id, ability_id)
);

CREATE TABLE IF NOT EXISTS compendium.class_features (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    class_id bigint NOT NULL REFERENCES compendium.classes (id) ON DELETE CASCADE,
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    ordering integer NOT NULL,
    UNIQUE (class_id, slug)
);

CREATE TABLE IF NOT EXISTS compendium.class_feature_levels (
    feature_id bigint NOT NULL REFERENCES compendium.class_features (id) ON DELETE CASCADE,
    level integer NOT NULL CHECK (level BETWEEN 1 AND 20),
    PRIMARY KEY (feature_id, level)
);

CREATE TABLE IF NOT EXISTS compendium.species (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    subspecies boolean NOT NULL,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.backgrounds (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.feats (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    feat_type text NOT NULL,
    prerequisite text NOT NULL,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.species_traits (
    species_id bigint NOT NULL REFERENCES compendium.species (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    PRIMARY KEY (species_id, ordering)
);

CREATE TABLE IF NOT EXISTS compendium.background_benefits (
    background_id bigint NOT NULL REFERENCES compendium.backgrounds (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    PRIMARY KEY (background_id, ordering)
);

CREATE TABLE IF NOT EXISTS compendium.feat_benefits (
    feat_id bigint NOT NULL REFERENCES compendium.feats (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    description text NOT NULL,
    PRIMARY KEY (feat_id, ordering)
);

CREATE TABLE IF NOT EXISTS compendium.weapons (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    damage_dice text NOT NULL,
    damage_type_id bigint REFERENCES compendium.damage_types (id),
    range_feet integer NOT NULL CHECK (range_feet >= 0),
    long_range_feet integer NOT NULL CHECK (long_range_feet >= 0),
    simple boolean NOT NULL,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.weapon_properties (
    weapon_id bigint NOT NULL REFERENCES compendium.weapons (id) ON DELETE CASCADE,
    name text NOT NULL,
    mastery boolean NOT NULL,
    detail text,
    PRIMARY KEY (weapon_id, name)
);

CREATE TABLE IF NOT EXISTS compendium.armor (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    category text NOT NULL,
    ac_base integer NOT NULL CHECK (ac_base BETWEEN 0 AND 30),
    add_dex boolean NOT NULL,
    dex_cap integer,
    stealth_disadvantage boolean NOT NULL,
    strength_required integer,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.items (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    category text NOT NULL,
    cost_gp numeric(12, 2) NOT NULL CHECK (cost_gp >= 0),
    weight_lb numeric(10, 3) NOT NULL CHECK (weight_lb >= 0),
    magic boolean NOT NULL,
    rarity text,
    requires_attunement boolean NOT NULL,
    attunement_detail text,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.monsters (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id bigint NOT NULL REFERENCES compendium.documents (id),
    slug text NOT NULL,
    name text NOT NULL,
    size text NOT NULL,
    creature_type text NOT NULL,
    alignment text NOT NULL,
    armor_class integer NOT NULL CHECK (armor_class BETWEEN 0 AND 40),
    armor_detail text,
    hit_points integer NOT NULL CHECK (hit_points > 0),
    hit_dice text NOT NULL,
    challenge_rating numeric(5, 3) NOT NULL CHECK (challenge_rating >= 0),
    xp integer NOT NULL CHECK (xp >= 0),
    strength integer NOT NULL,
    dexterity integer NOT NULL,
    constitution integer NOT NULL,
    intelligence integer NOT NULL,
    wisdom integer NOT NULL,
    charisma integer NOT NULL,
    passive_perception integer NOT NULL,
    languages text,
    UNIQUE (slug, document_id)
);

CREATE TABLE IF NOT EXISTS compendium.monster_stats (
    monster_id bigint NOT NULL REFERENCES compendium.monsters (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('save', 'skill', 'speed', 'sense')),
    name text NOT NULL,
    value integer NOT NULL,
    PRIMARY KEY (monster_id, kind, name)
);

CREATE TABLE IF NOT EXISTS compendium.monster_relations (
    monster_id bigint NOT NULL REFERENCES compendium.monsters (id) ON DELETE CASCADE,
    relation text NOT NULL CHECK (relation IN ('resistance', 'immunity', 'vulnerability', 'condition-immunity')),
    target_slug text NOT NULL,
    PRIMARY KEY (monster_id, relation, target_slug)
);

CREATE TABLE IF NOT EXISTS compendium.monster_traits (
    monster_id bigint NOT NULL REFERENCES compendium.monsters (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    PRIMARY KEY (monster_id, ordering)
);

CREATE TABLE IF NOT EXISTS compendium.monster_actions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    monster_id bigint NOT NULL REFERENCES compendium.monsters (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    action_type text NOT NULL,
    UNIQUE (monster_id, ordering)
);

CREATE TABLE IF NOT EXISTS compendium.monster_attacks (
    action_id bigint NOT NULL REFERENCES compendium.monster_actions (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    to_hit integer NOT NULL,
    reach_feet integer NOT NULL,
    range_feet integer NOT NULL,
    long_range_feet integer NOT NULL,
    damage_dice text,
    damage_bonus integer NOT NULL,
    damage_type text,
    extra_dice text,
    extra_type text,
    PRIMARY KEY (action_id, ordering)
);

-- One searchable list across every entry kind; the API blends rulesets over it.
CREATE OR REPLACE VIEW compendium.entries AS
    SELECT 'class'::text AS kind, c.id, c.slug, c.name, c.document_id,
           CASE WHEN c.parent_slug IS NULL THEN 'Class' ELSE 'Subclass of ' || initcap(replace(c.parent_slug, '-', ' ')) END AS subtitle
    FROM compendium.classes c
    UNION ALL
    SELECT 'species', s.id, s.slug, s.name, s.document_id, CASE WHEN s.subspecies THEN 'Subspecies' ELSE 'Species' END
    FROM compendium.species s
    UNION ALL
    SELECT 'background', b.id, b.slug, b.name, b.document_id, 'Background' FROM compendium.backgrounds b
    UNION ALL
    SELECT 'feat', f.id, f.slug, f.name, f.document_id, f.feat_type || ' feat' FROM compendium.feats f
    UNION ALL
    SELECT 'weapon', w.id, w.slug, w.name, w.document_id, w.damage_dice || CASE WHEN w.simple THEN ' · simple' ELSE ' · martial' END
    FROM compendium.weapons w
    UNION ALL
    SELECT 'armor', a.id, a.slug, a.name, a.document_id, initcap(a.category) || ' armour · AC ' || a.ac_base FROM compendium.armor a
    UNION ALL
    SELECT CASE WHEN i.magic THEN 'magic-item' ELSE 'item' END, i.id, i.slug, i.name, i.document_id,
           coalesce(initcap(i.rarity), initcap(replace(i.category, '-', ' ')))
    FROM compendium.items i
    UNION ALL
    SELECT 'monster', m.id, m.slug, m.name, m.document_id,
           'CR ' || rtrim(rtrim(m.challenge_rating::text, '0'), '.') || ' · ' || initcap(m.creature_type)
    FROM compendium.monsters m
    UNION ALL
    SELECT 'condition', c.id, c.slug, c.name, c.document_id, 'Condition' FROM compendium.conditions c;
