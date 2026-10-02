-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Which of a Character's two weapon sets it holds: melee (main and off hand) or ranged (the ranged slots).
ALTER TABLE campaign.characters ADD COLUMN IF NOT EXISTS weapon_set text NOT NULL DEFAULT 'melee';
ALTER TABLE campaign.characters ADD CONSTRAINT characters_weapon_set_check CHECK (weapon_set IN ('melee', 'ranged')) NOT VALID;

-- Weapons a Combatant may still equip or unequip with the attacks it has made this turn.
ALTER TABLE play.combatants ADD COLUMN IF NOT EXISTS equips integer NOT NULL DEFAULT 0;
ALTER TABLE play.combatants ADD CONSTRAINT combatants_equips_check CHECK (equips BETWEEN 0 AND 10) NOT VALID;
