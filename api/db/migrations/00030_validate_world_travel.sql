-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.maps VALIDATE CONSTRAINT maps_kind_check;
ALTER TABLE play.sessions VALIDATE CONSTRAINT sessions_world_map_fk;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
