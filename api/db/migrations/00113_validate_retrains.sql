-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.revisions VALIDATE CONSTRAINT revisions_entity_type_check;
