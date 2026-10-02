-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_components VALIDATE CONSTRAINT effect_components_kind_check;
