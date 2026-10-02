-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.map_objects VALIDATE CONSTRAINT map_objects_kind_check;
ALTER TABLE campaign.map_objects VALIDATE CONSTRAINT map_objects_trap_lock_check;
ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_action_check;
ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_target_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
