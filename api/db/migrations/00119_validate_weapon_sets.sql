-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.characters VALIDATE CONSTRAINT characters_weapon_set_check;
ALTER TABLE play.combatants VALIDATE CONSTRAINT combatants_equips_check;
