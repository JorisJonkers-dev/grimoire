-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE play.travel_legs VALIDATE CONSTRAINT travel_legs_vehicle_check;
