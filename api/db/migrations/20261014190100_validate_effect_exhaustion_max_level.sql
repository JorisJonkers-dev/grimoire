-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_exhaustion VALIDATE CONSTRAINT effect_exhaustion_max_level_check;
