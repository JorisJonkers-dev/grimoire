-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.combatants VALIDATE CONSTRAINT combatants_attacks_left_check;
ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_attacks_per_action_check;
ALTER TABLE play.token_attacks VALIDATE CONSTRAINT token_attacks_damage_mod_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
