-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- An Attack action may hold several attacks (Extra Attack), with movement between them; a Light weapon
-- attack opens an off-hand attack; each turn has one free object interaction.
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS attacks_left integer NOT NULL DEFAULT 0;
ALTER TABLE play.combatants ADD CONSTRAINT combatants_attacks_left_check CHECK (attacks_left BETWEEN 0 AND 10) NOT VALID;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS light_attack boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS off_hand boolean NOT NULL DEFAULT false;
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS interaction boolean NOT NULL DEFAULT true;

ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS attacks_per_action integer NOT NULL DEFAULT 1;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_attacks_per_action_check CHECK (attacks_per_action BETWEEN 1 AND 4) NOT VALID;

-- Whether a weapon attack is Light, and the ability modifier inside its damage bonus that an off-hand
-- attack leaves out.
ALTER TABLE play.token_attacks ADD COLUMN IF NOT EXISTS light boolean NOT NULL DEFAULT false;
ALTER TABLE play.token_attacks ADD COLUMN IF NOT EXISTS damage_mod integer NOT NULL DEFAULT 0;
ALTER TABLE play.token_attacks ADD CONSTRAINT token_attacks_damage_mod_check CHECK (damage_mod BETWEEN -5 AND 10) NOT VALID;

ALTER TABLE play.attacks ADD COLUMN IF NOT EXISTS off_hand boolean NOT NULL DEFAULT false;

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
    'action_taken', 'unarmed_strike', 'action_resolved', 'object_used')) NOT VALID;

-- Extra Attack: how many attacks one Attack action holds, by class level.
-- +goose StatementBegin
DO $$
DECLARE
    r bigint;
    c text;
BEGIN
    r := NULL;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('fighter-extra-attack', 'Extra Attack', 'class', 'fighter')
    ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 5, '2'), (r, 11, '3'), (r, 20, '4');
    END IF;
    FOREACH c IN ARRAY ARRAY['barbarian', 'monk', 'paladin', 'ranger'] LOOP
        r := NULL;
        INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES (c || '-extra-attack', 'Extra Attack', 'class', c)
        ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
        IF r IS NOT NULL THEN
            INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 5, '2');
        END IF;
    END LOOP;
END
$$;
-- +goose StatementEnd
