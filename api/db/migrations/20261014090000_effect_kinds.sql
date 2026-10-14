-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Every kind of Part the rules know, one row each. A kind's own migration adds its row next to its
-- table, so kinds no longer share one check constraint that every migration redefines. From here on
-- migrations take timestamp versions and may arrive out of order.
CREATE TABLE IF NOT EXISTS compendium.effect_kinds (
    slug text PRIMARY KEY CHECK (slug ~ '^[a-z]+(_[a-z]+)*$' AND char_length(slug) <= 40)
);

INSERT INTO compendium.effect_kinds (slug) VALUES ('area'), ('bonus_die'), ('branch'), ('choice'), ('counter'),
    ('create_surface'), ('crit_within'), ('dispel'), ('edge'), ('exhausting'), ('extra_damage'), ('forced_move'),
    ('form'), ('grant_feature'), ('immobile'), ('incapacitated'), ('manual'), ('mode'), ('move_cost'), ('reacts'),
    ('resource_change'), ('reveal'), ('save_condition'), ('save_damage'), ('save_edge'), ('speed_penalty'),
    ('summon'), ('teleport'), ('temp_hp')
ON CONFLICT (slug) DO NOTHING;

ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_kind_fkey
    FOREIGN KEY (kind) REFERENCES compendium.effect_kinds (slug) NOT VALID;
