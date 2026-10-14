-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Part beneath another names the group it sits in: "then" under a branch, a mode's name under a
-- choice. A Part at the top of its Effect sits in no group.
ALTER TABLE compendium.effect_components ADD COLUMN IF NOT EXISTS group_name text;
ALTER TABLE compendium.effect_components ADD CONSTRAINT effect_components_group_check CHECK (
    (parent IS NULL) = (group_name IS NULL) AND (group_name IS NULL OR char_length(group_name) BETWEEN 1 AND 80)
) NOT VALID;
