-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_game_day_check;
ALTER TABLE campaign.revisions VALIDATE CONSTRAINT revisions_entity_type_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
