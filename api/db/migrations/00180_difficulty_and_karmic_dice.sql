-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Campaign's difficulty preset, and whether the dice the server rolls are karmic.
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS difficulty text NOT NULL DEFAULT 'standard';
ALTER TABLE campaign.campaigns ADD COLUMN IF NOT EXISTS karmic_dice boolean NOT NULL DEFAULT false;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_difficulty_check
    CHECK (difficulty IN ('story', 'standard', 'hard')) NOT VALID;

-- The face a karmic d20 rolled and let go. Only a die the server rolled can have one: a die a player
-- threw is never karmic.
ALTER TABLE play.roll_dice ADD COLUMN IF NOT EXISTS karmic_dropped integer;
ALTER TABLE play.roll_dice ADD CONSTRAINT roll_dice_karmic_check
    CHECK (karmic_dropped IS NULL OR (mode = 'auto' AND faces = 20 AND karmic_dropped BETWEEN 1 AND 20)) NOT VALID;

-- A roll asked of its roller, by live play or by a DM, rather than one a Member made for themself.
-- Only an asked roll can be karmic or count towards a run, so nobody makes their own luck.
ALTER TABLE play.roll_requests ADD COLUMN IF NOT EXISTS asked boolean NOT NULL DEFAULT false;

-- Each roller's latest asked d20s rolled by the server, newest first: what karmic dice go by. Like a
-- Roll Request, it names its roller without holding them to the Campaign: a roll asked of a Member who
-- has since left still rolls.
CREATE TABLE IF NOT EXISTS play.roll_karma (
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    member_id uuid NOT NULL,
    recent integer[] NOT NULL,
    PRIMARY KEY (campaign_id, member_id),
    CONSTRAINT roll_karma_recent_check CHECK (cardinality(recent) BETWEEN 1 AND 8)
);
