-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- What kind of creature a token is (undead, fey, humanoid), so an Effect can touch only some kinds.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS creature_type text NOT NULL DEFAULT '';
ALTER TABLE play.tokens ADD CONSTRAINT tokens_creature_type_check CHECK (length(creature_type) <= 40) NOT VALID;
