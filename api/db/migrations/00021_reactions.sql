-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS reaction_timeout_s integer NOT NULL DEFAULT 10;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_reaction_timeout_check CHECK (reaction_timeout_s BETWEEN 3 AND 120) NOT VALID;

ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS can_shield boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS shielded boolean NOT NULL DEFAULT false;
ALTER TABLE play.combats ADD COLUMN IF NOT EXISTS resume_token_id uuid;
ALTER TABLE play.combats ADD COLUMN IF NOT EXISTS resume_cost_ft integer;
ALTER TABLE play.combats ADD CONSTRAINT combats_resume_token_fk FOREIGN KEY (resume_token_id)
    REFERENCES play.tokens (id) ON DELETE SET NULL NOT VALID;

-- The rest of a walk an opportunity attack interrupted, start first.
CREATE TABLE IF NOT EXISTS play.combat_resume_path (
    combat_id uuid NOT NULL REFERENCES play.combats (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    q integer NOT NULL,
    r integer NOT NULL,
    PRIMARY KEY (combat_id, ordering)
);

CREATE TABLE IF NOT EXISTS play.reaction_prompts (
    id uuid PRIMARY KEY,
    combat_id uuid NOT NULL UNIQUE REFERENCES play.combats (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('opportunity_attack', 'shield')),
    reactor_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    trigger_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    attack_no integer NOT NULL CHECK (attack_no >= 0),
    effect text NOT NULL,
    deadline timestamptz NOT NULL
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS reaction_prompts_reactor_idx ON play.reaction_prompts (reactor_token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS reaction_prompts_trigger_idx ON play.reaction_prompts (trigger_token_id);

ALTER TABLE play.attacks ADD COLUMN IF NOT EXISTS total integer;
ALTER TABLE play.attacks ADD COLUMN IF NOT EXISTS opportunity boolean NOT NULL DEFAULT false;
ALTER TABLE play.attacks DROP CONSTRAINT IF EXISTS attacks_stage_check;
ALTER TABLE play.attacks ADD CONSTRAINT attacks_stage_check CHECK (stage IN ('to_hit', 'reaction', 'damage')) NOT VALID;

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined')) NOT VALID;
