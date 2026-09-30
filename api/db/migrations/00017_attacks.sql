-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS stat_source text;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS armor_class integer;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS hp integer;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS hp_max integer;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_stats_check CHECK (
    (stat_source IS NULL AND armor_class IS NULL AND hp IS NULL AND hp_max IS NULL)
    OR (stat_source IS NOT NULL AND armor_class BETWEEN 0 AND 40 AND hp_max > 0 AND hp BETWEEN 0 AND hp_max)) NOT VALID;

CREATE TABLE IF NOT EXISTS play.token_attacks (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    to_hit integer NOT NULL CHECK (to_hit BETWEEN -10 AND 30),
    reach_ft integer NOT NULL CHECK (reach_ft BETWEEN 0 AND 60),
    range_ft integer NOT NULL CHECK (range_ft >= 0),
    long_range_ft integer NOT NULL CHECK (long_range_ft >= range_ft),
    damage_dice text NOT NULL,
    damage_bonus integer NOT NULL CHECK (damage_bonus BETWEEN -10 AND 50),
    damage_type text NOT NULL,
    PRIMARY KEY (token_id, ordering)
);

CREATE TABLE IF NOT EXISTS play.attacks (
    id uuid PRIMARY KEY,
    combat_id uuid NOT NULL UNIQUE REFERENCES play.combats (id) ON DELETE CASCADE,
    attacker_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    target_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    attack_no integer NOT NULL CHECK (attack_no >= 0),
    mode text NOT NULL CHECK (mode IN ('normal', 'advantage', 'disadvantage')),
    cover_bonus integer NOT NULL CHECK (cover_bonus BETWEEN 0 AND 5),
    stage text NOT NULL CHECK (stage IN ('to_hit', 'damage')),
    critical boolean NOT NULL,
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id) ON DELETE CASCADE
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS attacks_roll_idx ON play.attacks (roll_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS attacks_attacker_idx ON play.attacks (attacker_token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS attacks_target_idx ON play.attacks (target_token_id);

CREATE TABLE IF NOT EXISTS play.action_hp_events (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    token_id uuid NOT NULL,
    hp_before integer NOT NULL,
    hp_after integer NOT NULL,
    undoes_action_id uuid UNIQUE REFERENCES play.actions (id) ON DELETE CASCADE
);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone')) NOT VALID;
