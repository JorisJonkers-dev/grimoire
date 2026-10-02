-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_creation_methods_check;
ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_starting_level_check;
ALTER TABLE campaign.account_characters VALIDATE CONSTRAINT account_characters_appearance_check;
