-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Dice Set: the look an Account gives its dice, per die type, with at most one uploaded picture placed
-- on them. A copy is a set someone took of a set shared with them: it cannot be edited, and it stays
-- when the sharing stops. A set shared with everyone that carries a picture waits for an Admin's review.
CREATE TABLE IF NOT EXISTS social.dice_sets (
    id uuid PRIMARY KEY,
    owner_account uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    design jsonb NOT NULL CHECK (jsonb_typeof(design) = 'object'),
    image_key text,
    image_type text,
    sharing text NOT NULL CHECK (sharing IN ('private', 'friends', 'everyone')),
    review text NOT NULL CHECK (review IN ('none', 'pending', 'approved', 'rejected')),
    copied_from uuid,
    made_by text NOT NULL CHECK (char_length(made_by) BETWEEN 1 AND 60),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK ((image_key IS NULL) = (image_type IS NULL)),
    CHECK (copied_from IS NULL OR sharing = 'private')
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS dice_sets_owner_idx ON social.dice_sets (owner_account);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS dice_sets_one_copy_idx ON social.dice_sets (owner_account, copied_from) WHERE copied_from IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS dice_sets_shared_idx ON social.dice_sets (sharing) WHERE sharing <> 'private';

-- The Dice Set an Account rolls with; no row means the plain dice.
CREATE TABLE IF NOT EXISTS social.dice_set_choices (
    account_id uuid PRIMARY KEY REFERENCES identity.accounts (id) ON DELETE CASCADE,
    dice_set_id uuid NOT NULL REFERENCES social.dice_sets (id) ON DELETE CASCADE
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS dice_set_choices_set_idx ON social.dice_set_choices (dice_set_id);
