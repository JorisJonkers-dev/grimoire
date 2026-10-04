-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_difficulty_check;
ALTER TABLE play.roll_dice VALIDATE CONSTRAINT roll_dice_karmic_check;
