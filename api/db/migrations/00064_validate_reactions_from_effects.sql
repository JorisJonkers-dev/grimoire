-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_components VALIDATE CONSTRAINT effect_components_kind_check;
ALTER TABLE play.reaction_prompts VALIDATE CONSTRAINT reaction_prompts_kind_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
