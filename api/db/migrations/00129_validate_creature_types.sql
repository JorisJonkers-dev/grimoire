-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_creature_type_check;
