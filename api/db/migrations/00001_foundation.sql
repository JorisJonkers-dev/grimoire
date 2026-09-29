-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

CREATE SCHEMA IF NOT EXISTS compendium;
CREATE SCHEMA IF NOT EXISTS campaign;
CREATE SCHEMA IF NOT EXISTS prep;
CREATE SCHEMA IF NOT EXISTS play;
CREATE SCHEMA IF NOT EXISTS ops;

CREATE TABLE IF NOT EXISTS ops.instance (
    id bigint PRIMARY KEY CHECK (id = 1),
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO ops.instance (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
