-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A line the DM puts on the Table Display, under the map: read aloud, or a place name.
ALTER TABLE play.table_displays ADD COLUMN IF NOT EXISTS caption text NOT NULL DEFAULT '';
ALTER TABLE play.table_displays DROP CONSTRAINT IF EXISTS table_displays_caption_check;
ALTER TABLE play.table_displays ADD CONSTRAINT table_displays_caption_check CHECK (char_length(caption) <= 200) NOT VALID;
