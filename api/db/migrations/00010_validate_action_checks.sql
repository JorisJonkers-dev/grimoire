-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_seed_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_session_fk;
