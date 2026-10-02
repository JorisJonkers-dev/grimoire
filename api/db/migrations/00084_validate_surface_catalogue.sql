-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.surfaces VALIDATE CONSTRAINT surfaces_kind_check;
ALTER TABLE compendium.effect_surfaces VALIDATE CONSTRAINT effect_surfaces_surface_check;
