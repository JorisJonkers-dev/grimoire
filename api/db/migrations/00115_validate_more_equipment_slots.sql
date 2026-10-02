-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.item_instances VALIDATE CONSTRAINT item_instances_equipped_slot_check;
