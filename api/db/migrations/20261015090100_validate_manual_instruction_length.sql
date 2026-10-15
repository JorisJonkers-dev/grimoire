-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE compendium.effect_manual VALIDATE CONSTRAINT effect_manual_instruction_length;
