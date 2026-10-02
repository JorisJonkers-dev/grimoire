-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Reactions from any trigger: an Effect can give its bearer a reaction the DM resolves, and each
-- Controller says per kind of reaction whether to ask, always take it, or never.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_reactions (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'reacts' CHECK (kind = 'reacts'),
    trigger text NOT NULL CHECK (trigger IN ('damaged')),
    instruction text NOT NULL CHECK (char_length(instruction) BETWEEN 1 AND 500),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

ALTER TABLE play.reaction_prompts DROP CONSTRAINT IF EXISTS reaction_prompts_kind_check;
ALTER TABLE play.reaction_prompts ADD CONSTRAINT reaction_prompts_kind_check
    CHECK (kind IN ('opportunity_attack', 'shield', 'readied', 'effect')) NOT VALID;

CREATE TABLE IF NOT EXISTS play.token_reactions (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('opportunity_attack', 'shield', 'readied', 'effect')),
    mode text NOT NULL CHECK (mode IN ('ask', 'always', 'never')),
    condition text NOT NULL CHECK (condition IN ('', 'target_bloodied')),
    PRIMARY KEY (token_id, kind)
);

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set')) NOT VALID;

-- Hellish Rebuke, the SRD reaction to being damaged.
-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
BEGIN
    INSERT INTO compendium.effect_definitions (slug, name, concentration, owner_kind, owner_slug)
    VALUES ('hellish-rebuke', 'Hellish Rebuke', false, 'spell', 'hellish-rebuke') ON CONFLICT (slug) DO NOTHING RETURNING id INTO e;
    IF e IS NOT NULL THEN
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, 0, 'reacts');
        INSERT INTO compendium.effect_reactions (effect_id, ordinal, trigger, instruction)
        VALUES (e, 0, 'damaged', 'Hellish Rebuke: the creature that damaged you makes a Dexterity save or takes 2d10 fire damage, half on a success.');
    END IF;
END
$$;
-- +goose StatementEnd
