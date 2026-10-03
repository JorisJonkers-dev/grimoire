-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE library.entries VALIDATE CONSTRAINT entries_kind_check;
ALTER TABLE library.proposals VALIDATE CONSTRAINT proposals_kind_check;
ALTER TABLE library.shared_submissions VALIDATE CONSTRAINT shared_submissions_kind_check;
ALTER TABLE campaign.campaigns VALIDATE CONSTRAINT campaigns_exhaustion_variant_check;
