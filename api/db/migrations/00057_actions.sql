-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The 2024 actions: a readied attack waits on its trigger, Disengage keeps a mover safe for the turn,
-- Dash may double movement, and Grapple, Shove and Hide wait on their rolls.
ALTER TABLE play.reaction_prompts DROP CONSTRAINT IF EXISTS reaction_prompts_kind_check;
ALTER TABLE play.reaction_prompts ADD CONSTRAINT reaction_prompts_kind_check
    CHECK (kind IN ('opportunity_attack', 'shield', 'readied')) NOT VALID;

ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS disengaged boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS readied_trigger text;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS readied_who uuid;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS readied_attack integer;
ALTER TABLE play.combatants ADD CONSTRAINT combatants_readied_check CHECK (
    (readied_trigger IS NULL OR readied_trigger IN ('enters_reach')) AND (readied_trigger IS NULL) = (readied_attack IS NULL)
    AND (readied_attack IS NULL OR readied_attack >= 0)) NOT VALID;
ALTER TABLE play.combatants DROP CONSTRAINT IF EXISTS combatants_movement_ft_check;
ALTER TABLE play.combatants ADD CONSTRAINT combatants_movement_ft_check CHECK (movement_ft BETWEEN 0 AND 240) NOT VALID;

-- The saving throw a creature's Grapple or Shove forces.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS unarmed_dc integer NOT NULL DEFAULT 10;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_unarmed_dc_check CHECK (unarmed_dc BETWEEN 1 AND 40) NOT VALID;

CREATE TABLE IF NOT EXISTS play.pending_actions (
    roll_id uuid PRIMARY KEY REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES play.sessions (id) ON DELETE CASCADE,
    actor_token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    target_token_id uuid REFERENCES play.tokens (id) ON DELETE CASCADE,
    action text NOT NULL CHECK (action IN ('hide', 'grapple', 'shove_push', 'shove_prone')),
    dc integer NOT NULL CHECK (dc BETWEEN 1 AND 40),
    CHECK ((action = 'hide') = (target_token_id IS NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS pending_actions_session_idx ON play.pending_actions (session_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS pending_actions_actor_idx ON play.pending_actions (actor_token_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS pending_actions_target_idx ON play.pending_actions (target_token_id);

ALTER TABLE play.actions DROP CONSTRAINT IF EXISTS actions_kind_check;
ALTER TABLE play.actions ADD CONSTRAINT actions_kind_check CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered',
    'roll_resolved', 'session_started', 'session_ended', 'token_placed', 'token_moved', 'token_hidden', 'token_revealed',
    'token_removed', 'map_set', 'hexes_revealed', 'hexes_concealed', 'walls_set', 'walls_cleared', 'light_placed',
    'light_removed', 'ambient_set', 'token_walked', 'combat_started', 'initiative_rolled', 'turn_ended', 'resource_spent',
    'combat_ended', 'attack_declared', 'attack_missed', 'attack_hit', 'damage_dealt', 'damage_undone', 'tactics_set',
    'reaction_offered', 'reaction_used', 'reaction_declined', 'effect_applied', 'effect_ended', 'save_passed', 'save_failed',
    'manual_resolved', 'area_cast', 'area_resolved', 'surfaces_set', 'elevation_set', 'table_set', 'world_set', 'node_added',
    'node_removed', 'route_added', 'route_removed', 'party_placed', 'travel_leg', 'zone_added',
    'zone_removed', 'zone_held', 'zone_sprung', 'perception_rolled',
    'rest_taken', 'check_scheduled', 'encounter_checked', 'encounter_resolved', 'loot_dropped',
    'item_moved', 'coins_moved', 'shop_opened',
    'shop_closed', 'item_bought', 'item_sold', 'haggle_started', 'haggled', 'stock_rolled',
    'encounter_spawned', 'hp_adjusted',
    'rest_proposed', 'rest_agreed', 'rest_started', 'hit_die_spent', 'hit_die_healed', 'rest_interrupted',
    'action_taken', 'unarmed_strike', 'action_resolved')) NOT VALID;

-- Dodge as an Effect: attacks against the dodger have disadvantage, and it saves Dexterity with advantage.
-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('dodging', 'Dodging', false, 'feature', 'dodge') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'edge'), (e, 1, 'save_edge');
        INSERT INTO compendium.effect_edges (effect_id, ordinal, against, advantage, reach) VALUES (e, 0, true, false, 'any');
        INSERT INTO compendium.effect_save_edges (effect_id, ordinal, ability, mode) VALUES (e, 1, 'dexterity', 'advantage');
    END IF;
END
$$;
-- +goose StatementEnd
