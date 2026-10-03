-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.characters VALIDATE CONSTRAINT characters_downtime_check;
ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_downtime_advanced_check;
