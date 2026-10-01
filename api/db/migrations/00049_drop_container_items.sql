-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Every stack has been an Item Instance since the backfill; nothing reads the slug-keyed rows.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS campaign.container_items;
