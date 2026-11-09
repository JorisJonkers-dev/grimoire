-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Where an Admin emailed an Account Invite's link; NULL when the Admin passed the link on themselves.
ALTER TABLE identity.invites ADD COLUMN IF NOT EXISTS email text;
ALTER TABLE identity.invites ADD CONSTRAINT invites_email_check CHECK (char_length(email) BETWEEN 3 AND 254 AND email LIKE '%_@_%') NOT VALID;
