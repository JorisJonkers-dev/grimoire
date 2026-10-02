-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Visibility Qualities on tokens, the Senses that get past them, and Reveal Effects that strip them.
ALTER TABLE compendium.effect_components DROP CONSTRAINT IF EXISTS effect_components_kind_check;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_check CHECK (kind IN ('bonus_die', 'edge',
    'extra_damage', 'move_cost', 'manual', 'area', 'save_damage', 'save_condition', 'create_surface', 'incapacitated',
    'immobile', 'save_edge', 'crit_within', 'exhausting', 'speed_penalty', 'reacts', 'temp_hp', 'teleport', 'forced_move',
    'dispel', 'counter', 'grant_feature', 'resource_change', 'choice', 'mode', 'branch', 'summon', 'form', 'reveal')) NOT VALID;

CREATE TABLE IF NOT EXISTS compendium.effect_reveals (
    effect_id bigint NOT NULL,
    ordinal integer NOT NULL,
    kind text NOT NULL DEFAULT 'reveal' CHECK (kind = 'reveal'),
    qualities text[] NOT NULL CHECK (cardinality(qualities) BETWEEN 1 AND 8 AND qualities <@ ARRAY['hidden', 'invisible', 'disguised', 'illusory', 'ethereal', 'darkness', 'heavy', 'secret']::text[]),
    PRIMARY KEY (effect_id, ordinal),
    FOREIGN KEY (effect_id, ordinal, kind) REFERENCES compendium.effect_components (effect_id, ordinal, kind) ON DELETE CASCADE
);

-- A Disguised token shows this name to anyone who has not seen through it.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS disguise text;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_disguise_check CHECK (disguise IS NULL OR char_length(disguise) BETWEEN 1 AND 40) NOT VALID;

CREATE TABLE IF NOT EXISTS play.token_qualities (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    quality text NOT NULL CHECK (quality IN ('hidden', 'invisible', 'disguised', 'illusory', 'ethereal', 'darkness', 'heavy', 'secret')),
    seen_through boolean NOT NULL DEFAULT false,
    PRIMARY KEY (token_id, quality)
);

CREATE TABLE IF NOT EXISTS play.token_senses (
    token_id uuid NOT NULL REFERENCES play.tokens (id) ON DELETE CASCADE,
    sense text NOT NULL CHECK (sense IN ('blindsight', 'tremorsense', 'truesight')),
    range_ft integer NOT NULL CHECK (range_ft BETWEEN 5 AND 1000),
    PRIMARY KEY (token_id, sense)
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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used', 'mastery_used', 'reaction_set', 'concentration_checked', 'downed', 'dying_changed', 'revived', 'teleported', 'countered', 'summoned', 'commanded', 'visibility_set')) NOT VALID;

-- Faerie Fire outlines what is in it: nothing there benefits from being Invisible.
-- +goose StatementBegin
DO $$
DECLARE
    e bigint;
    n integer;
BEGIN
    SELECT d.id INTO e FROM compendium.effect_definitions d WHERE d.slug = 'faerie-fire'
        AND NOT EXISTS (SELECT 1 FROM compendium.effect_components c WHERE c.effect_id = d.id AND c.kind = 'reveal');
    IF e IS NOT NULL THEN
        SELECT coalesce(max(ordinal), -1) + 1 INTO n FROM compendium.effect_components WHERE effect_id = e;
        INSERT INTO compendium.effect_components (effect_id, ordinal, kind) VALUES (e, n, 'reveal');
        INSERT INTO compendium.effect_reveals (effect_id, ordinal, qualities) VALUES (e, n, ARRAY['invisible']);
    END IF;
END
$$;
-- +goose StatementEnd
