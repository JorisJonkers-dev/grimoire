-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_reaction_timeout_check;
ALTER TABLE play.combats VALIDATE CONSTRAINT combats_resume_token_fk;
ALTER TABLE play.attacks VALIDATE CONSTRAINT attacks_stage_check;
ALTER TABLE play.actions VALIDATE CONSTRAINT actions_kind_check;
