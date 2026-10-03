-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.map_nodes VALIDATE CONSTRAINT map_nodes_local_map_fk;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
