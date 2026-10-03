-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.pending_actions VALIDATE CONSTRAINT pending_actions_action_check;
