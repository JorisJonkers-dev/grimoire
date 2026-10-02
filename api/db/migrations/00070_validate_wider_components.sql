-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_components VALIDATE CONSTRAINT effect_components_kind_check;
ALTER TABLE compendium.effect_areas VALIDATE CONSTRAINT effect_areas_shape_check;
ALTER TABLE play.tokens VALIDATE CONSTRAINT tokens_temp_hp_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
