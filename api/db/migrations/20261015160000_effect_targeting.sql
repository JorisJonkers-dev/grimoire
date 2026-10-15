-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What an Effect's Activation may pick: self, chosen creatures or objects, a point, or an area its area
-- Part places; how many, how far (0 is touch), from which side, of which creature types, whether each
-- must be seen or willing, and how far each may be from the first. No row states no Targeting.
CREATE TABLE IF NOT EXISTS compendium.effect_targeting (
    effect_id bigint PRIMARY KEY REFERENCES compendium.effect_definitions (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('self', 'creatures', 'object', 'point', 'area')),
    target_count integer NOT NULL CHECK (target_count BETWEEN 0 AND 50),
    range_ft integer NOT NULL CHECK (range_ft BETWEEN 0 AND 5280),
    side text NOT NULL CHECK (side IN ('', 'allies', 'enemies')),
    creature_types text[] NOT NULL CHECK (cardinality(creature_types) <= 14),
    seen boolean NOT NULL,
    willing boolean NOT NULL,
    spread_ft integer NOT NULL CHECK (spread_ft BETWEEN 0 AND 5280)
);
