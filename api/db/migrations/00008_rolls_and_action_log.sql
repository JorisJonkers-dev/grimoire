-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The Action Log is append-only. Actor columns copy the member's name, so the log survives membership changes.
CREATE TABLE IF NOT EXISTS play.actions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    seq bigint NOT NULL CHECK (seq >= 1),
    kind text NOT NULL CHECK (kind IN ('roll_requested', 'die_rolled', 'die_entered', 'roll_resolved')),
    actor_member_id uuid NOT NULL,
    actor_name text NOT NULL,
    origin text NOT NULL CHECK (origin IN ('ui', 'mcp', 'generator', 'system')),
    client text NOT NULL DEFAULT '',
    seed bigint,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, seq),
    CHECK ((kind = 'die_rolled') = (seed IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS play.roll_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    purpose text NOT NULL CHECK (char_length(purpose) BETWEEN 1 AND 120),
    notation text NOT NULL CHECK (char_length(notation) BETWEEN 3 AND 80),
    requested_by_name text NOT NULL,
    roller_member_id uuid NOT NULL,
    roller_subject text NOT NULL,
    roller_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'resolved')),
    total integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    CHECK ((status = 'resolved') = (total IS NOT NULL AND resolved_at IS NOT NULL))
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS roll_requests_campaign_idx ON play.roll_requests (campaign_id, created_at DESC);

CREATE TABLE IF NOT EXISTS play.roll_request_labels (
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    group_no integer NOT NULL CHECK (group_no >= 0),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    PRIMARY KEY (roll_id, group_no)
);

CREATE TABLE IF NOT EXISTS play.roll_request_modifiers (
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    ordering integer NOT NULL CHECK (ordering >= 0),
    label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
    value integer NOT NULL CHECK (value BETWEEN -100 AND 100),
    PRIMARY KEY (roll_id, ordering)
);

CREATE TABLE IF NOT EXISTS play.roll_dice (
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    die_no integer NOT NULL CHECK (die_no >= 0),
    group_no integer NOT NULL CHECK (group_no >= 0),
    faces integer NOT NULL CHECK (faces IN (4, 6, 8, 10, 12, 20, 100)),
    value integer CHECK (value BETWEEN 1 AND faces),
    mode text CHECK (mode IN ('auto', 'manual')),
    PRIMARY KEY (roll_id, die_no),
    CHECK ((value IS NULL) = (mode IS NULL))
);

CREATE TABLE IF NOT EXISTS play.action_roll_events (
    action_id uuid PRIMARY KEY REFERENCES play.actions (id) ON DELETE CASCADE,
    roll_id uuid NOT NULL REFERENCES play.roll_requests (id) ON DELETE CASCADE,
    die_no integer,
    value integer NOT NULL
);
