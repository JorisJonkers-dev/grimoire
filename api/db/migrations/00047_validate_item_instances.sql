-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.containers VALIDATE CONSTRAINT containers_parent_fk;
ALTER TABLE campaign.containers VALIDATE CONSTRAINT containers_kind_check;
ALTER TABLE campaign.containers VALIDATE CONSTRAINT containers_one_owner_check;
