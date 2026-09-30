-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Effects on tokens in a live Session: modelled ones fold into rolls and movement; the rest are tracked
-- and resolved by the DM from Manual prompts.
CREATE TABLE IF NOT EXISTS play.active_effects (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    target_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    source_token_id uuid REFERENCES play.tokens (id) ON DELETE SET NULL,
    slug text NOT NULL CHECK (char_length(slug) BETWEEN 1 AND 80),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    concentration boolean NOT NULL,
    rounds_left integer CHECK (rounds_left > 0),
    save_ability text CHECK (save_ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    save_dc integer CHECK (save_dc BETWEEN 1 AND 40),
    CHECK ((save_ability IS NULL) = (save_dc IS NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS active_effects_session_idx ON play.active_effects (session_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS active_effects_target_idx ON play.active_effects (target_token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS active_effects_source_idx ON play.active_effects (source_token_id);

CREATE TABLE IF NOT EXISTS play.manual_prompts (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    ordering integer NOT NULL,
    text text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 300)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS manual_prompts_session_idx ON play.manual_prompts (session_id);

CREATE TABLE IF NOT EXISTS play.token_saves (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    ability text NOT NULL CHECK (ability IN ('strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma')),
    bonus integer NOT NULL CHECK (bonus BETWEEN -10 AND 30),
    PRIMARY KEY (token_id, ability)
);

-- Saving throws a bearer rolls to end an effect.
CREATE TABLE IF NOT EXISTS play.pending_saves (
    roll_id uuid PRIMARY KEY REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    effect_id uuid NOT NULL REFERENCES play.active_effects (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    dc integer NOT NULL CHECK (dc BETWEEN 1 AND 40)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS pending_saves_effect_idx ON play.pending_saves (effect_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS pending_saves_session_idx ON play.pending_saves (session_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved')) NOT VALID;
