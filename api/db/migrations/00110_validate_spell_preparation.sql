-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_game_minute_check;
