-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.character_abilities VALIDATE CONSTRAINT character_abilities_increase_check;
