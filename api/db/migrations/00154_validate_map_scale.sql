-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE campaign.maps VALIDATE CONSTRAINT maps_grid_kind_check;
ALTER TABLE campaign.maps VALIDATE CONSTRAINT maps_grid_strength_check;
ALTER TABLE campaign.maps VALIDATE CONSTRAINT maps_scale_miles_check;
