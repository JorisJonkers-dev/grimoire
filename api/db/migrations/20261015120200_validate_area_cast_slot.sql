-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.area_casts VALIDATE CONSTRAINT area_casts_slot_level_check;
