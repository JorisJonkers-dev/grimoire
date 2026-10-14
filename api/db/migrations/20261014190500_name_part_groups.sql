-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Backfill: the Parts under a branch sit in its group "then"; the Parts under a mode move up to the
-- choice, in a group named after the mode, and the mode rows go.
UPDATE compendium.effect_components c SET group_name = 'then'
FROM compendium.effect_components b
WHERE b.effect_id = c.effect_id AND b.ordinal = c.parent AND b.kind = 'branch' AND c.group_name IS NULL;

UPDATE compendium.effect_components c SET parent = m.parent, group_name = n.name
FROM compendium.effect_components m
JOIN compendium.effect_modes n ON n.effect_id = m.effect_id AND n.ordinal = m.ordinal
WHERE m.effect_id = c.effect_id AND m.ordinal = c.parent AND m.kind = 'mode';

DELETE FROM compendium.effect_components WHERE kind = 'mode';

DELETE FROM compendium.effect_kinds WHERE slug = 'mode';
