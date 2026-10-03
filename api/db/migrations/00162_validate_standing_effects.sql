-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_faction_fk;
ALTER TABLE prep.shops VALIDATE CONSTRAINT shops_faction_fk;
ALTER TABLE prep.table_entries VALIDATE CONSTRAINT table_entries_faction_fk;
