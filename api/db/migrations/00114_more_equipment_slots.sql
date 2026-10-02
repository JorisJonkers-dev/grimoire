-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Ammunition and an instrument get slots of their own beside the worn and held ones.
ALTER TABLE campaign.item_instances DROP CONSTRAINT IF EXISTS item_instances_equipped_slot_check;
ALTER TABLE campaign.item_instances ADD CONSTRAINT item_instances_equipped_slot_check
    CHECK (equipped_slot IN ('main_hand', 'off_hand', 'ranged_main', 'ranged_off', 'armor', 'head', 'cloak',
        'hands', 'feet', 'neck', 'ring_1', 'ring_2', 'ammunition', 'instrument')) NOT VALID;
