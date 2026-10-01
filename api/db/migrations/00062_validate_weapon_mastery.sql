-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_components VALIDATE CONSTRAINT effect_components_kind_check;
ALTER TABLE play.token_attacks VALIDATE CONSTRAINT token_attacks_mastery_check;
ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_action_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
