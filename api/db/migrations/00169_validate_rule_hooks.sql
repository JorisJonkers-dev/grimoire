-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_hook_name_check;
ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_dc_check;
ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_action_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
