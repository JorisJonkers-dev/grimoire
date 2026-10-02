-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.characters VALIDATE CONSTRAINT characters_character_fk;
ALTER TABLE campaign.characters VALIDATE CONSTRAINT characters_character_set;
