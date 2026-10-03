-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.table_displays VALIDATE CONSTRAINT table_displays_caption_check;
