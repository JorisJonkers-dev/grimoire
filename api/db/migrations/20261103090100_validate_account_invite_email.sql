-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE identity.invites VALIDATE CONSTRAINT invites_email_check;
