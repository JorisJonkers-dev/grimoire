-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.characters VALIDATE CONSTRAINT characters_hit_dice_spent_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
