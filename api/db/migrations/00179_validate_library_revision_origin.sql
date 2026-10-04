-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE library.entry_revisions VALIDATE CONSTRAINT entry_revisions_origin_check;
