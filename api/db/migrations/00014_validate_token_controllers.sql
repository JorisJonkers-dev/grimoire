-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_controller_fk;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
