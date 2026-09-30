-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS intelligence integer;
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS tactics text NOT NULL DEFAULT 'auto';
ALTER TABLE play.tokens ADD CONSTRAINT tokens_tactics_check CHECK (tactics IN ('auto', 'simple', 'cunning', 'off')) NOT VALID;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_intelligence_check CHECK (intelligence BETWEEN 1 AND 30) NOT VALID;
ALTER TABLE play.attacks ADD COLUMN IF NOT EXISTS ranged boolean NOT NULL DEFAULT false;

-- What each creature has seen others deal from range; Cunning creatures go after the worst of them.
CREATE TABLE IF NOT EXISTS play.observed_damage (
    observer_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    attacker_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    ranged_damage integer NOT NULL CHECK (ranged_damage >= 0),
    PRIMARY KEY (observer_token_id, attacker_token_id)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS observed_damage_attacker_idx ON play.observed_damage (attacker_token_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set')) NOT VALID;
