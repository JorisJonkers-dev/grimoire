-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.sessions VALIDATE CONSTRAINT sessions_map_fk;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
