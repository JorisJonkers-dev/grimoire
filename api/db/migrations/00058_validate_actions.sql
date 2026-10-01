-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.reaction_prompts VALIDATE CONSTRAINT reaction_prompts_kind_check;
ALTER TABLE play.combatants VALIDATE CONSTRAINT combatants_readied_check;
ALTER TABLE play.combatants VALIDATE CONSTRAINT combatants_movement_ft_check;
ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_unarmed_dc_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
